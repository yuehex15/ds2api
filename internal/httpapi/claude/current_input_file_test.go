package claude

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ds2api/internal/auth"
	dsclient "ds2api/internal/deepseek/client"
)

type claudeCurrentInputAuth struct{}

func (claudeCurrentInputAuth) Determine(*http.Request) (*auth.RequestAuth, error) {
	return &auth.RequestAuth{
		DeepSeekToken: "direct-token",
		CallerID:      "caller:test",
		TriedAccounts: map[string]bool{},
	}, nil
}

func (claudeCurrentInputAuth) Release(*auth.RequestAuth) {}

type claudeCurrentInputDS struct {
	uploads []dsclient.UploadFileRequest
	payload map[string]any
}

func (d *claudeCurrentInputDS) CreateSession(context.Context, *auth.RequestAuth, int) (string, error) {
	return "session-id", nil
}

func (d *claudeCurrentInputDS) GetPow(context.Context, *auth.RequestAuth, int) (string, error) {
	return "pow", nil
}

func (d *claudeCurrentInputDS) UploadFile(_ context.Context, _ *auth.RequestAuth, req dsclient.UploadFileRequest, _ int) (*dsclient.UploadFileResult, error) {
	d.uploads = append(d.uploads, req)
	return &dsclient.UploadFileResult{ID: "file-claude-history"}, nil
}

func (d *claudeCurrentInputDS) CallCompletion(_ context.Context, _ *auth.RequestAuth, payload map[string]any, _ string, _ int) (*http.Response, error) {
	d.payload = payload
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("data: {\"p\":\"response/content\",\"v\":\"ok\"}\n")),
	}, nil
}

func TestClaudeDirectAppliesCurrentInputFile(t *testing.T) {
	ds := &claudeCurrentInputDS{}
	h := &Handler{
		Store: mockClaudeConfig{aliases: map[string]string{"claude-sonnet-4-6": "deepseek-v4-flash"}},
		Auth:  claudeCurrentInputAuth{},
		DS:    ds,
	}
	reqBody := `{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hello from claude"}],"max_tokens":1024}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Messages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(ds.uploads) != 1 {
		t.Fatalf("expected one current input upload, got %d", len(ds.uploads))
	}
	if ds.uploads[0].Filename != "DS2API_HISTORY.txt" {
		t.Fatalf("unexpected upload filename: %q", ds.uploads[0].Filename)
	}
	refIDs, _ := ds.payload["ref_file_ids"].([]any)
	if len(refIDs) != 1 || refIDs[0] != "file-claude-history" {
		t.Fatalf("expected uploaded history ref id, got %#v", ds.payload["ref_file_ids"])
	}
	prompt, _ := ds.payload["prompt"].(string)
	if !strings.Contains(prompt, "Continue from the latest state in the attached DS2API_HISTORY.txt context.") {
		t.Fatalf("expected continuation prompt, got %q", prompt)
	}
}
