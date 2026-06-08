package palm2

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"chat/utils"
	"fmt"
	"strings"
)

type ImageProps struct {
	Model  string
	Prompt string
	Proxy  globals.ProxyConfig
}

func (c *ChatInstance) GetImageEndpoint(model string) string {
	return fmt.Sprintf("%s/v1beta/models/%s:predict?key=%s", c.Endpoint, model, c.ApiKey)
}

func (c *ChatInstance) GetGeminiImageBody(props ImageProps) *GeminiChatBody {
	return &GeminiChatBody{
		Contents: c.GetGeminiContents(props.Model, []globals.Message{{
			Role:    globals.User,
			Content: props.Prompt,
		}}),
		GenerationConfig: GeminiConfig{
			ResponseModalities: []string{"IMAGE", "TEXT"},
		},
	}
}

// CreateImageRequest will create a gemini imagen from prompt, return base64 of image and error
func (c *ChatInstance) CreateImageRequest(props ImageProps) (string, error) {
	if globals.IsGeminiImageModel(props.Model) {
		res, err := utils.Post(
			c.GetChatEndpoint(props.Model, false),
			map[string]string{
				"Content-Type": "application/json",
			},
			c.GetGeminiImageBody(props),
			props.Proxy,
		)

		if err != nil || res == nil {
			return "", fmt.Errorf("gemini error: %s", err.Error())
		}

		return c.GetGeminiImageResponse(res)
	}

	res, err := utils.Post(
		c.GetImageEndpoint(props.Model),
		map[string]string{
			"Content-Type": "application/json",
		},
		ImageRequest{
			Instances: []ImageInstance{
				{
					Prompt: props.Prompt,
				},
			},
			Parameters: ImageParameters{
				SampleCount:      1,
				AspectRatio:      "1:1",
				PersonGeneration: "allow_adult",
			},
		},
		props.Proxy,
	)

	if err != nil || res == nil {
		return "", fmt.Errorf("gemini error: %s", err.Error())
	}

	data := utils.MapToStruct[ImageResponse](res)
	if data == nil {
		return "", fmt.Errorf("gemini error: cannot parse response")
	}

	if len(data.Predictions) == 0 {
		return "", fmt.Errorf("gemini error: no image generated")
	}

	return data.Predictions[0].BytesBase64Encoded, nil
}

func (c *ChatInstance) GetGeminiImageResponse(data interface{}) (string, error) {
	if form := utils.MapToStruct[GeminiChatResponse](data); form != nil {
		if len(form.Candidates) == 0 || len(form.Candidates[0].Content.Parts) == 0 {
			return "", fmt.Errorf("gemini error: no image generated")
		}

		var textParts []string
		for _, part := range form.Candidates[0].Content.Parts {
			if part.InlineData != nil && strings.HasPrefix(strings.ToLower(part.InlineData.MimeType), "image/") && part.InlineData.Data != "" {
				return fmt.Sprintf("data:%s;base64,%s", part.InlineData.MimeType, part.InlineData.Data), nil
			}
			if part.Text != nil && strings.TrimSpace(*part.Text) != "" {
				textParts = append(textParts, strings.TrimSpace(*part.Text))
			}
		}

		if len(textParts) > 0 {
			return "", fmt.Errorf("gemini error: no image generated (%s)", strings.Join(textParts, " "))
		}
	}

	if form := utils.MapToStruct[GeminiChatErrorResponse](data); form != nil {
		return "", fmt.Errorf("gemini error: %s (code: %d, status: %s)", form.Error.Message, form.Error.Code, form.Error.Status)
	}

	return "", fmt.Errorf("gemini error: cannot parse image response")
}

// CreateImage will create a gemini imagen from prompt, return markdown of image
func (c *ChatInstance) CreateImage(props *adaptercommon.ChatProps) (string, error) {
	if !globals.IsGoogleImagenModel(props.Model) {
		return "", nil
	}

	base64Data, err := c.CreateImageRequest(ImageProps{
		Model:  props.Model,
		Prompt: c.GetLatestPrompt(props),
		Proxy:  props.Proxy,
	})

	if err != nil {
		if strings.Contains(err.Error(), "safety") {
			return err.Error(), nil
		}
		return "", err
	}

	dataURL := base64Data
	if !strings.HasPrefix(dataURL, "data:image/") {
		dataURL = fmt.Sprintf("data:image/png;base64,%s", base64Data)
	}

	url := utils.StoreImage(dataURL)
	return utils.GetImageMarkdown(url), nil
}
