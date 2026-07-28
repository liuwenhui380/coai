package azure

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"encoding/json"
	"strings"
	"testing"
)

func TestGetChatBodyOmitsSamplingControlsForAllModels(t *testing.T) {
	value := float32(0.7)
	instance := NewChatInstance("2024-02-15-preview", "test-key", "https://example.openai.azure.com")

	body := instance.GetChatBody(&adaptercommon.ChatProps{
		OriginalModel:    "gpt-4o",
		Model:            "gpt-4o",
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
	for _, field := range []string{"temperature", "top_p", "presence_penalty", "frequency_penalty"} {
		if strings.Contains(string(raw), field) {
			t.Fatalf("Azure request JSON must not contain %s: %s", field, raw)
		}
	}
}
