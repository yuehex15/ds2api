package toolcall

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
)

var xmlToolCallsWrapperPattern = regexp.MustCompile(`(?is)<tool_calls\b[^>]*>\s*(.*?)\s*</tool_calls>`)
var xmlInvokePattern = regexp.MustCompile(`(?is)<invoke\b([^>]*)>\s*(.*?)\s*</invoke>`)
var xmlParameterPattern = regexp.MustCompile(`(?is)<parameter\b([^>]*)>\s*(.*?)\s*</parameter>`)
var xmlAttrPattern = regexp.MustCompile(`(?is)\b([a-z0-9_:-]+)\s*=\s*("([^"]*)"|'([^']*)')`)

func parseXMLToolCalls(text string) []ParsedToolCall {
	wrappers := xmlToolCallsWrapperPattern.FindAllStringSubmatch(text, -1)
	if len(wrappers) == 0 {
		return nil
	}
	out := make([]ParsedToolCall, 0, len(wrappers))
	for _, wrapper := range wrappers {
		if len(wrapper) < 2 {
			continue
		}
		for _, block := range xmlInvokePattern.FindAllStringSubmatch(wrapper[1], -1) {
			call, ok := parseSingleXMLToolCall(block)
			if !ok {
				continue
			}
			out = append(out, call)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseSingleXMLToolCall(block []string) (ParsedToolCall, bool) {
	if len(block) < 3 {
		return ParsedToolCall{}, false
	}
	attrs := parseXMLTagAttributes(block[1])
	name := strings.TrimSpace(html.UnescapeString(attrs["name"]))
	if name == "" {
		return ParsedToolCall{}, false
	}

	inner := strings.TrimSpace(block[2])
	if strings.HasPrefix(inner, "{") {
		var payload map[string]any
		if err := json.Unmarshal([]byte(inner), &payload); err == nil {
			input := map[string]any{}
			if params, ok := payload["input"].(map[string]any); ok {
				input = params
			}
			if len(input) == 0 {
				if params, ok := payload["parameters"].(map[string]any); ok {
					input = params
				}
			}
			return ParsedToolCall{Name: name, Input: input}, true
		}
	}

	input := map[string]any{}
	for _, paramMatch := range xmlParameterPattern.FindAllStringSubmatch(inner, -1) {
		if len(paramMatch) < 3 {
			continue
		}
		paramAttrs := parseXMLTagAttributes(paramMatch[1])
		paramName := strings.TrimSpace(html.UnescapeString(paramAttrs["name"]))
		if paramName == "" {
			continue
		}
		value := parseInvokeParameterValue(paramMatch[2])
		appendMarkupValue(input, paramName, value)
	}

	if len(input) == 0 {
		if strings.TrimSpace(inner) != "" {
			return ParsedToolCall{}, false
		}
		return ParsedToolCall{Name: name, Input: map[string]any{}}, true
	}
	return ParsedToolCall{Name: name, Input: input}, true
}

func parseXMLTagAttributes(raw string) map[string]string {
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}
	}
	out := map[string]string{}
	for _, m := range xmlAttrPattern.FindAllStringSubmatch(raw, -1) {
		if len(m) < 5 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(m[1]))
		if key == "" {
			continue
		}
		value := m[3]
		if value == "" {
			value = m[4]
		}
		out[key] = value
	}
	return out
}

func parseInvokeParameterValue(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if parsed := parseStructuredToolCallInput(trimmed); len(parsed) > 0 {
		if len(parsed) == 1 {
			if rawValue, ok := parsed["_raw"].(string); ok {
				return rawValue
			}
		}
		return parsed
	}
	return html.UnescapeString(extractRawTagValue(trimmed))
}
