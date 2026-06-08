package openai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"testing"
)

func TestGetChatBodyIncludesRoutingUserAndMetadata(t *testing.T) {
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
