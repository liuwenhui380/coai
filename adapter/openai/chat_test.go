package openai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetChatBodyOmitsSamplingControlsForClaudeCompatibleModels(t *testing.T) {
	temperature := float32(0.7)
	topP := float32(0.9)
	instance := NewChatInstance("https://example.com", "test-key")

	body := instance.GetChatBody(&adaptercommon.ChatProps{
		OriginalModel: "claude-opus-4-7",
		Model:         "claude-opus-4-7",
		Message:       []globals.Message{{Role: globals.User, Content: "hello"}},
		Temperature:   &temperature,
		TopP:          &topP,
	}, false)

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "temperature") {
		t.Fatalf("Claude-compatible OpenAI request JSON should not contain temperature: %s", raw)
	}
	if strings.Contains(string(raw), "top_p") {
		t.Fatalf("Claude-compatible OpenAI request JSON should not contain top_p: %s", raw)
	}
}

func TestGetChatBodyIncludesUserAndMetadata(t *testing.T) {
	instance := NewChatInstance("https://example.com", "test-key")

	body := instance.GetChatBody(&adaptercommon.ChatProps{
		Model:   "chatgpt-pro",
		Message: []globals.Message{{Role: globals.User, Content: "hello"}},
		User:    "chatnio-1234567890abcdef",
		Metadata: map[string]interface{}{
			"chatnio_user_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}, false)

	request, ok := body.(ChatRequest)
	if !ok {
		t.Fatalf("GetChatBody returned %T, want ChatRequest", body)
	}
	if request.User != "chatnio-1234567890abcdef" {
		t.Fatalf("user = %q", request.User)
	}
	if request.Metadata["chatnio_user_hash"] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("metadata = %#v", request.Metadata)
	}
}

func TestCreateChatRequestDelegatesGeminiImageModelsToNativeGenerateContent(t *testing.T) {
	oldAcceptImageStore := globals.AcceptImageStore
	oldNotifyURL := globals.NotifyUrl
	globals.AcceptImageStore = false
	globals.NotifyUrl = ""
	defer func() {
		globals.AcceptImageStore = oldAcceptImageStore
		globals.NotifyUrl = oldNotifyURL
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-3-pro-image-preview:generateContent" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("key"); got != "test-key" {
			t.Fatalf("unexpected api key query: %q", got)
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"inline_data":{"mime_type":"image/png","data":"aGVsbG8="}}]}}]}`))
	}))
	defer server.Close()

	instance := NewChatInstance(server.URL, "test-key")
	content, err := instance.CreateChatRequest(&adaptercommon.ChatProps{
		Model: "gemini-3-pro-image-preview",
		Message: []globals.Message{{
			Role:    globals.User,
			Content: "draw a cat",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "data:image/png;base64,aGVsbG8=") {
		t.Fatalf("unexpected content: %s", content)
	}
}
