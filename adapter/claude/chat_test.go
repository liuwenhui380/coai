package claude

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"encoding/json"
	"strings"
	"testing"
)

func TestGetChatBodyIgnoresSamplingControls(t *testing.T) {
	temperature := float32(0.7)
	topP := float32(0.9)
	topK := 5
	instance := NewChatInstance("https://api.anthropic.com", "test-key")
	props := &adaptercommon.ChatProps{
		Model:       "claude-3-5-sonnet",
		Message:     []globals.Message{{Role: globals.User, Content: "hello"}},
		Temperature: &temperature,
		TopP:        &topP,
		TopK:        &topK,
	}

	body := instance.GetChatBody(props, false)

	if body.Temperature != nil {
		t.Fatalf("temperature should be omitted for Claude requests, got %v", *body.Temperature)
	}
	if body.TopP != nil {
		t.Fatalf("top_p should be omitted for Claude requests, got %v", *body.TopP)
	}
	if body.TopK != nil {
		t.Fatalf("top_k should be omitted for Claude requests, got %v", *body.TopK)
	}

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"temperature", "top_p", "top_k"} {
		if strings.Contains(string(raw), field) {
			t.Fatalf("Claude request JSON should not contain %s: %s", field, raw)
		}
	}
}
