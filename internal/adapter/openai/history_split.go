package openai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ds2api/internal/auth"
	"ds2api/internal/deepseek"
	"ds2api/internal/util"
)

const (
	historySplitFilename         = "HISTORY.txt"
	historySplitInjectedFilename = "IGNORE"
	historySplitContentType      = "text/plain; charset=utf-8"
	historySplitPurpose          = "assistants"
)

func (h *Handler) applyHistorySplit(ctx context.Context, a *auth.RequestAuth, stdReq util.StandardRequest) (util.StandardRequest, error) {
	if h == nil || h.DS == nil || h.Store == nil || a == nil {
		return stdReq, nil
	}
	if !h.Store.HistorySplitEnabled() {
		return stdReq, nil
	}

	promptMessages, historyMessages := splitOpenAIHistoryMessages(stdReq.Messages, h.Store.HistorySplitTriggerAfterTurns())
	if len(historyMessages) == 0 {
		return stdReq, nil
	}

	historyText := buildOpenAIHistoryTranscript(historyMessages)
	if strings.TrimSpace(historyText) == "" {
		return stdReq, errors.New("history split produced empty transcript")
	}

	result, err := h.DS.UploadFile(ctx, a, deepseek.UploadFileRequest{
		Filename:    historySplitFilename,
		ContentType: historySplitContentType,
		Purpose:     historySplitPurpose,
		Data:        []byte(historyText),
	}, 3)
	if err != nil {
		return stdReq, fmt.Errorf("upload history file: %w", err)
	}
	fileID := strings.TrimSpace(result.ID)
	if fileID == "" {
		return stdReq, errors.New("upload history file returned empty file id")
	}

	stdReq.Messages = promptMessages
	stdReq.HistoryText = historyText
	stdReq.RefFileIDs = prependUniqueRefFileID(stdReq.RefFileIDs, fileID)
	stdReq.FinalPrompt, stdReq.ToolNames = buildOpenAIFinalPromptWithPolicy(promptMessages, stdReq.ToolsRaw, "", stdReq.ToolChoice, stdReq.Thinking)
	return stdReq, nil
}

func splitOpenAIHistoryMessages(messages []any, triggerAfterTurns int) ([]any, []any) {
	if triggerAfterTurns <= 0 {
		triggerAfterTurns = 1
	}
	lastUserIndex := -1
	userTurns := 0
	for i, raw := range messages {
		msg, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(asString(msg["role"])))
		if role != "user" {
			continue
		}
		userTurns++
		lastUserIndex = i
	}
	if userTurns <= triggerAfterTurns || lastUserIndex < 0 {
		return messages, nil
	}

	promptMessages := make([]any, 0, len(messages)-lastUserIndex)
	historyMessages := make([]any, 0, lastUserIndex)
	for i, raw := range messages {
		msg, ok := raw.(map[string]any)
		if !ok {
			if i >= lastUserIndex {
				promptMessages = append(promptMessages, raw)
			} else {
				historyMessages = append(historyMessages, raw)
			}
			continue
		}
		role := strings.ToLower(strings.TrimSpace(asString(msg["role"])))
		switch role {
		case "system", "developer":
			promptMessages = append(promptMessages, raw)
		default:
			if i >= lastUserIndex {
				promptMessages = append(promptMessages, raw)
			} else {
				historyMessages = append(historyMessages, raw)
			}
		}
	}
	if len(promptMessages) == 0 {
		return messages, nil
	}
	return promptMessages, historyMessages
}

func buildOpenAIHistoryTranscript(messages []any) string {
	normalized := normalizeOpenAIMessagesForPrompt(messages, "")
	transcript := strings.TrimSpace(deepseek.MessagesPrepare(normalized))
	if transcript == "" {
		return ""
	}
	return fmt.Sprintf("[file content end]\n\n%s\n\n[file name]: %s\n[file content begin]\n", transcript, historySplitInjectedFilename)
}

func prependUniqueRefFileID(existing []string, fileID string) []string {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return existing
	}
	out := make([]string, 0, len(existing)+1)
	out = append(out, fileID)
	for _, id := range existing {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" || strings.EqualFold(trimmed, fileID) {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
