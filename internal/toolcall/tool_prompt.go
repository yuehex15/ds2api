package toolcall

import "strings"

// BuildToolCallInstructions generates the unified tool-calling instruction block
// used by all adapters (OpenAI, Claude, Gemini). It uses attention-optimized
// structure: rules → negative examples → positive examples → anchor.
//
// The toolNames slice should contain the actual tool names available in the
// current request; the function picks real names for examples.
func BuildToolCallInstructions(toolNames []string) string {
	// Pick real tool names for examples; fall back to generic names.
	ex1 := "read_file"
	ex2 := "write_to_file"
	ex3 := "ask_followup_question"
	used := map[string]bool{}
	for _, n := range toolNames {
		switch {
		// Read/query-type tools
		case !used["ex1"] && matchAny(n, "read_file", "list_files", "search_files", "Read", "Glob"):
			ex1 = n
			used["ex1"] = true
		// Write/execute-type tools
		case !used["ex2"] && matchAny(n, "write_to_file", "apply_diff", "execute_command", "exec_command", "Write", "Edit", "MultiEdit", "Bash"):
			ex2 = n
			used["ex2"] = true
		// Interactive/meta tools
		case !used["ex3"] && matchAny(n, "ask_followup_question", "attempt_completion", "update_todo_list", "Task"):
			ex3 = n
			used["ex3"] = true
		}
	}
	ex1Params := exampleReadParams(ex1)
	ex2Params := exampleWriteOrExecParams(ex2)
	ex3Params := exampleInteractiveParams(ex3)

	return `TOOL CALL FORMAT — FOLLOW EXACTLY:

<tool_calls>
  <invoke name="TOOL_NAME_HERE">
    <parameter name="PARAMETER_NAME"><![CDATA[PARAMETER_VALUE]]></parameter>
  </invoke>
</tool_calls>

RULES:
1) Use the <tool_calls> XML wrapper format only.
2) Put one or more <invoke> entries under a single <tool_calls> root.
3) Put the tool name in the invoke name attribute: <invoke name="TOOL_NAME">.
4) All string values must use <![CDATA[...]]>, even short ones. This includes code, scripts, file contents, prompts, paths, names, and queries.
5) Every top-level argument must be a <parameter name="ARG_NAME">...</parameter> node.
6) Objects use nested XML elements inside the parameter body. Arrays may repeat <item> children.
7) Numbers, booleans, and null stay plain text.
8) Use only the parameter names in the tool schema. Do not invent fields.
9) Do NOT wrap XML in markdown fences. Do NOT output explanations, role markers, or internal monologue.

PARAMETER SHAPES:
- string => <parameter name="x"><![CDATA[value]]></parameter>
- object => <parameter name="x"><field>...</field></parameter>
- array => <parameter name="x"><item>...</item><item>...</item></parameter>
- number/bool/null => <parameter name="x">plain_text</parameter>

【WRONG — Do NOT do these】:

Wrong 1 — mixed text after XML:
  <tool_calls>...</tool_calls> I hope this helps.
Wrong 2 — Markdown code fences:
  ` + "```xml" + `
  <tool_calls>...</tool_calls>
  ` + "```" + `

Remember: The ONLY valid way to use tools is the <tool_calls>...</tool_calls> XML block at the end of your response.

【CORRECT EXAMPLES】:

Example A — Single tool:
<tool_calls>
  <invoke name="` + ex1 + `">
` + indentPromptParameters(ex1Params, "    ") + `
  </invoke>
</tool_calls>

Example B — Two tools in parallel:
<tool_calls>
  <invoke name="` + ex1 + `">
` + indentPromptParameters(ex1Params, "    ") + `
  </invoke>
  <invoke name="` + ex2 + `">
` + indentPromptParameters(ex2Params, "    ") + `
  </invoke>
  <invoke name="Read">
    <parameter name="file_path">` + promptCDATA("/abs/path/to/another-file.txt") + `</parameter>
  </invoke>
</tool_calls>

Example C — Tool with nested XML parameters:
<tool_calls>
  <invoke name="` + ex3 + `">
` + indentPromptParameters(ex3Params, "    ") + `
  </invoke>
</tool_calls>

Example D — Tool with long script using CDATA (RELIABLE FOR CODE/SCRIPTS):
<tool_calls>
  <invoke name="` + ex2 + `">
    <parameter name="path">` + promptCDATA("script.sh") + `</parameter>
    <parameter name="content"><![CDATA[
#!/bin/bash
if [ "$1" == "test" ]; then
  echo "Success!"
fi
]]></parameter>
  </invoke>
</tool_calls>

`
}

func indentPromptParameters(body, indent string) string {
	if strings.TrimSpace(body) == "" {
		return indent + `<parameter name="content"></parameter>`
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = line
			continue
		}
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

func wrapParameter(name, inner string) string {
	return `<parameter name="` + name + `">` + inner + `</parameter>`
}

func exampleReadParams(name string) string {
	switch strings.TrimSpace(name) {
	case "Read":
		return wrapParameter("file_path", promptCDATA("README.md"))
	case "Glob":
		return wrapParameter("pattern", promptCDATA("**/*.go")) + "\n" + wrapParameter("path", promptCDATA("."))
	default:
		return wrapParameter("path", promptCDATA("src/main.go"))
	}
}

func exampleWriteOrExecParams(name string) string {
	switch strings.TrimSpace(name) {
	case "Bash", "execute_command":
		return wrapParameter("command", promptCDATA("pwd"))
	case "exec_command":
		return wrapParameter("cmd", promptCDATA("pwd"))
	case "Edit":
		return wrapParameter("file_path", promptCDATA("README.md")) + "\n" + wrapParameter("old_string", promptCDATA("foo")) + "\n" + wrapParameter("new_string", promptCDATA("bar"))
	case "MultiEdit":
		return wrapParameter("file_path", promptCDATA("README.md")) + "\n" + `<parameter name="edits"><item><old_string>` + promptCDATA("foo") + `</old_string><new_string>` + promptCDATA("bar") + `</new_string></item></parameter>`
	default:
		return wrapParameter("path", promptCDATA("output.txt")) + "\n" + wrapParameter("content", promptCDATA("Hello world"))
	}
}

func exampleInteractiveParams(name string) string {
	switch strings.TrimSpace(name) {
	case "Task":
		return wrapParameter("description", promptCDATA("Investigate flaky tests")) + "\n" + wrapParameter("prompt", promptCDATA("Run targeted tests and summarize failures"))
	default:
		return wrapParameter("question", promptCDATA("Which approach do you prefer?")) + "\n" + `<parameter name="follow_up"><item><text>` + promptCDATA("Option A") + `</text></item><item><text>` + promptCDATA("Option B") + `</text></item></parameter>`
	}
}

func matchAny(name string, candidates ...string) bool {
	for _, c := range candidates {
		if name == c {
			return true
		}
	}
	return false
}

func promptCDATA(text string) string {
	if text == "" {
		return ""
	}
	if strings.Contains(text, "]]>") {
		return "<![CDATA[" + strings.ReplaceAll(text, "]]>", "]]]]><![CDATA[>") + "]]>"
	}
	return "<![CDATA[" + text + "]]>"
}
