package azure

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"encoding/json"
	"strings"
	"testing"
)

func TestGetChatBodyOmitsSamplingControlsForClaudeCompatibleModels(t *testing.T) {
	temperature := float32(0.7)
	topP := float32(0.9)
	instance := NewChatInstance("2024-02-15-preview", "test-key", "https://example.openai.azure.com")

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
		t.Fatalf("Claude-compatible Azure request JSON should not contain temperature: %s", raw)
	}
	if strings.Contains(string(raw), "top_p") {
		t.Fatalf("Claude-compatible Azure request JSON should not contain top_p: %s", raw)
	}
}
