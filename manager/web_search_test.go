package manager

import (
	"chat/globals"
	"testing"
)

func TestShouldSkipWebSearchForImageGenerationModel(t *testing.T) {
	if !shouldSkipWebSearch(globals.GPTImage2, []globals.Message{{Role: globals.User, Content: "draw a logo"}}) {
		t.Fatal("image generation models should not wait on web search")
	}
}

func TestShouldSkipWebSearchForImageContent(t *testing.T) {
	messages := []globals.Message{{
		Role:    globals.User,
		Content: "describe this data:image/png;base64,iVBORw0KGgo=",
	}}
	if !shouldSkipWebSearch("gpt-5.5", messages) {
		t.Fatal("messages with image data should not be sent to web search")
	}
}

func TestShouldAllowWebSearchForTextOnlyModel(t *testing.T) {
	messages := []globals.Message{{Role: globals.User, Content: "latest OpenAI news"}}
	if shouldSkipWebSearch("gpt-5.5", messages) {
		t.Fatal("text-only prompts should still allow web search")
	}
}
