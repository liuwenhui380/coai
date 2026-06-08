package globals

import "testing"

func TestIsVisionModelRecognizesModernMultimodalModels(t *testing.T) {
	models := []string{
		"gpt-5.5",
		"gpt-4.1",
		"o4-mini",
		"gemini-2.0-flash",
		"claude-sonnet-4-20250514",
	}

	for _, model := range models {
		if !IsVisionModel(model) {
			t.Fatalf("IsVisionModel(%q) = false, want true", model)
		}
	}
}

func TestIsOpenAIImageGenerationModelRecognizesGPTImage2(t *testing.T) {
	if !IsOpenAIImageGenerationModel(GPTImage2) {
		t.Fatal("gpt-image-2 should use the OpenAI Images API")
	}

	if !IsOpenAIImageGenerationModel("x-ai/grok-imagine-image") {
		t.Fatal("Grok Imagine Image should use the OpenAI Images API")
	}

	if IsOpenAIImageGenerationModel(GPT4Dalle) {
		t.Fatal("gpt-4-dalle should keep the legacy chat-compatible path")
	}
}

func TestIsImageGenerationModelUsesMarketTags(t *testing.T) {
	old := ImageGenerationModels
	ImageGenerationModels = []string{"custom-art-model"}
	defer func() { ImageGenerationModels = old }()

	if !IsImageGenerationModel("custom-art-model") {
		t.Fatal("market-tagged image generation model should be recognized")
	}
}

func TestIsGoogleImagenModelRecognizesGeminiImageModels(t *testing.T) {
	if !IsGoogleImagenModel("gemini-3.1-flash-image-preview") {
		t.Fatal("gemini image preview models should use the Gemini image path")
	}
}

func TestIsClaudeModelRecognizesAliases(t *testing.T) {
	models := []string{
		"claude-opus-4-7",
		"claude-sonnet-4-20250514",
		"anthropic/claude-3-5-sonnet",
	}

	for _, model := range models {
		if !IsClaudeModel(model) {
			t.Fatalf("IsClaudeModel(%q) = false, want true", model)
		}
	}
}
