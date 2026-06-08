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
	instance := NewChatInstance("https://api.anthropic.com", "test-key")
	props := &adaptercommon.ChatProps{
		Model:       "claude-3-5-sonnet",
		Message:     []globals.Message{{Role: globals.User, Content: "hello"}},
		Temperature: &temperature,
		TopP:        &topP,
	}

	body := instance.GetChatBody(props, false)

	if body.Temperature != nil {
		t.Fatalf("temperature should be omitted for Claude requests, got %v", *body.Temperature)
	}
	if body.TopP != nil {
		t.Fatalf("top_p should be omitted for Claude requests, got %v", *body.TopP)
	}

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "top_p") {
		t.Fatalf("Claude request JSON should not contain top_p: %s", raw)
	}
	if strings.Contains(string(raw), "temperature") {
		t.Fatalf("Claude request JSON should not contain temperature: %s", raw)
	}
}
