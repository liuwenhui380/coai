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

func TestGetChatBodyOmitsSamplingControlsForAllModels(t *testing.T) {
	value := float32(0.7)
	instance := NewChatInstance("https://example.com", "test-key")
	models := []struct {
		name          string
		originalModel string
		model         string
	}{
		{name: "ordinary compatible model", originalModel: "gpt-4o", model: "gpt-4o"},
		{name: "new OpenAI model", originalModel: "gpt-5.5", model: "gpt-5.5"},
		{name: "Claude compatible model", originalModel: "claude-opus-4-7", model: "claude-opus-4-7"},
		{name: "original k3 model", originalModel: "k3-256k", model: "mapped-k3-model"},
		{name: "mapped kimi coding model", originalModel: "internal-kimi-alias", model: "kimi-for-coding"},
	}

	for _, item := range models {
		t.Run(item.name, func(t *testing.T) {
			body := instance.GetChatBody(&adaptercommon.ChatProps{
				OriginalModel:    item.originalModel,
				Model:            item.model,
				Message:          []globals.Message{{Role: globals.User, Content: "hello"}},
				Temperature:      &value,
				TopP:             &value,
				PresencePenalty:  &value,
				FrequencyPenalty: &value,
			}, false)

			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			var request map[string]interface{}
			if err := json.Unmarshal(raw, &request); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"temperature", "top_p", "presence_penalty", "frequency_penalty"} {
				if _, ok := request[field]; ok {
					t.Fatalf("%s request JSON must not contain %s: %s", item.name, field, raw)
				}
			}
		})
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

func TestGetChatBodyOmitsChatnioIdentityForCompatibleUpstreams(t *testing.T) {
	instance := NewChatInstance("https://example.com", "test-key")
	models := []struct {
		name          string
		originalModel string
		model         string
	}{
		{name: "kimi", originalModel: "kimi-k2.5", model: "kimi-k2.5"},
		{name: "claude", originalModel: "claude-opus-4-8", model: "anthropic/claude-opus-4-8"},
	}

	for _, item := range models {
		t.Run(item.name, func(t *testing.T) {
			body := instance.GetChatBody(&adaptercommon.ChatProps{
				OriginalModel: item.originalModel,
				Model:         item.model,
				Message:       []globals.Message{{Role: globals.User, Content: "hello"}},
				User:          "chatnio-1234567890abcdef",
				Metadata: map[string]interface{}{
					"chatnio_user_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					"chat_id":           "42",
				},
			}, false)

			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			var request map[string]interface{}
			if err := json.Unmarshal(raw, &request); err != nil {
				t.Fatal(err)
			}
			if _, ok := request["user"]; ok {
				t.Fatalf("%s request JSON must not contain ChatNio user identity: %s", item.name, raw)
			}
			if _, ok := request["metadata"]; ok {
				t.Fatalf("%s request JSON must not contain ChatNio metadata: %s", item.name, raw)
			}
		})
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
