package utils

import (
	"chat/globals"
	"strings"
	"testing"
)

func TestTrimMessagesByTokenLimitTruncatesSingleOversizedMessage(t *testing.T) {
	const maxTokens = 64000
	originalContent := strings.Repeat("hello ", 70000)
	messages := []globals.Message{
		{
			Role:    globals.User,
			Content: originalContent,
		},
	}

	trimmed := TrimMessagesByTokenLimit(messages, globals.GPT3Turbo, maxTokens, 10)

	if len(trimmed) != 1 {
		t.Fatalf("len(trimmed) = %d, want 1", len(trimmed))
	}
	if trimmed[0].Content == originalContent {
		t.Fatal("content was not truncated")
	}
	if messages[0].Content != originalContent {
		t.Fatal("original message was mutated")
	}
	if tokens := NumTokensFromMessages(trimmed, globals.GPT3Turbo, false); tokens > maxTokens {
		t.Fatalf("tokens = %d, want <= %d", tokens, maxTokens)
	}
}

func TestTrimMessagesByTokenLimitPreservesBase64Images(t *testing.T) {
	image := "data:image/png;base64," + strings.Repeat("A", 4096)
	originalContent := strings.Repeat("hello ", 70000) + "![image](" + image + ")" + strings.Repeat("world ", 70000)
	messages := []globals.Message{
		{
			Role:    globals.User,
			Content: originalContent,
		},
	}

	trimmed := TrimMessagesByTokenLimit(messages, globals.GPT4O, 2000, 10)

	if len(trimmed) != 1 {
		t.Fatalf("len(trimmed) = %d, want 1", len(trimmed))
	}
	if !strings.Contains(trimmed[0].Content, image) {
		t.Fatal("base64 image data was truncated or removed")
	}
	if trimmed[0].Content == originalContent {
		t.Fatal("text content was not truncated")
	}
	if messages[0].Content != originalContent {
		t.Fatal("original message was mutated")
	}
	if tokens := numTokensFromMessagesForTokenLimit(trimmed, globals.GPT4O, false); tokens > 2000 {
		t.Fatalf("tokens = %d, want <= 2000", tokens)
	}
}

func TestBase64ImageCountsAsFixedTokenEstimateForLimit(t *testing.T) {
	image := "data:image/png;base64," + strings.Repeat("A", 4096)
	withoutImage := []globals.Message{{Role: globals.User, Content: "describe this image"}}
	withImage := []globals.Message{{Role: globals.User, Content: "describe this image" + image}}

	delta := numTokensFromMessagesForTokenLimit(withImage, globals.GPT4O, false) -
		numTokensFromMessagesForTokenLimit(withoutImage, globals.GPT4O, false)

	if delta != base64ImageTokenEstimate {
		t.Fatalf("base64 image token delta = %d, want %d", delta, base64ImageTokenEstimate)
	}
}

func TestBase64ImageTokenEstimateSupportsURLSafeDataURL(t *testing.T) {
	image := "data:image/svg+xml;name=test;base64," + strings.Repeat("A-_", 1024)
	messages := []globals.Message{{Role: globals.User, Content: "describe" + image}}

	tokens := numTokensFromMessagesForTokenLimit(messages, globals.GPT4O, false)
	textTokens := numTokensFromMessagesForTokenLimit([]globals.Message{{Role: globals.User, Content: "describe"}}, globals.GPT4O, false)

	if tokens-textTokens != base64ImageTokenEstimate {
		t.Fatalf("base64 image token delta = %d, want %d", tokens-textTokens, base64ImageTokenEstimate)
	}
}
