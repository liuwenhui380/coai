package openai

import (
	adaptercommon "chat/adapter/common"
	"chat/adapter/palm2"
	"chat/globals"
	"chat/utils"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

func (c *ChatInstance) GetChatEndpoint(props *adaptercommon.ChatProps) string {
	if props.Model == globals.GPT3TurboInstruct {
		return fmt.Sprintf("%s/v1/completions", c.GetEndpoint())
	}
	return fmt.Sprintf("%s/v1/chat/completions", c.GetEndpoint())
}

func (c *ChatInstance) GetCompletionPrompt(messages []globals.Message) string {
	result := ""
	for _, message := range messages {
		result += fmt.Sprintf("%s: %s\n", message.Role, message.Content)
	}
	return result
}

func (c *ChatInstance) GetLatestPrompt(props *adaptercommon.ChatProps) string {
	if len(props.Message) == 0 {
		return ""
	}

	return props.Message[len(props.Message)-1].Content
}

func isChatGPTWebModel(models ...string) bool {
	for _, model := range models {
		model = strings.ToLower(strings.TrimSpace(model))
		if strings.HasPrefix(model, "chatgpt-") {
			return true
		}
	}
	return false
}

func (c *ChatInstance) GetChatBody(props *adaptercommon.ChatProps, stream bool) interface{} {
	if props.Model == globals.GPT3TurboInstruct {
		// for completions
		return CompletionRequest{
			Model:    props.Model,
			Prompt:   c.GetCompletionPrompt(props.Message),
			MaxToken: props.MaxTokens,
			Stream:   stream,
		}
	}

	messages := formatMessages(props)

	// o1, o3, gpt-5 compatibility
	isNewModel := len(props.Model) >= 2 && (props.Model[:2] == "o1" || props.Model[:2] == "o3") || strings.HasPrefix(props.Model, "gpt-5")

	var temperature *float32
	if globals.IsClaudeModel(props.OriginalModel, props.Model) {
		temperature = nil
	} else if isNewModel {
		temp := float32(1.0)
		temperature = &temp
	} else {
		temperature = props.Temperature
	}
	topP := props.TopP
	if globals.IsClaudeModel(props.OriginalModel, props.Model) {
		topP = nil
	}

	request := ChatRequest{
		Model:            props.Model,
		Messages:         messages,
		Stream:           stream,
		PresencePenalty:  props.PresencePenalty,
		FrequencyPenalty: props.FrequencyPenalty,
		Temperature:      temperature,
		TopP:             topP,
		Tools:            props.Tools,
		ToolChoice:       props.ToolChoice,
	}

	// ChatNio 身份字段只供 ChatGPT-Mirror 做会话隔离，不能泄漏给严格校验请求体的兼容上游。
	if isChatGPTWebModel(props.OriginalModel, props.Model) {
		request.User = props.User
		request.Metadata = props.Metadata
	}

	if isNewModel {
		request.MaxCompletionTokens = props.MaxTokens
	} else {
		request.MaxToken = props.MaxTokens
	}
	return request
}

// CreateChatRequest is the native http request body for openai
func (c *ChatInstance) CreateChatRequest(props *adaptercommon.ChatProps) (string, error) {
	if globals.IsGeminiImageModel(props.Model) {
		return palm2.NewChatInstance(c.GetEndpoint(), c.GetApiKey()).CreateImage(props)
	}

	if globals.IsOpenAIImageGenerationModel(props.OriginalModel, props.Model) {
		return c.CreateImage(props)
	}

	res, err := utils.Post(
		c.GetChatEndpoint(props),
		c.GetHeader(),
		c.GetChatBody(props, false),
		props.Proxy,
	)

	if err != nil || res == nil {
		return "", fmt.Errorf("openai error: %s", err.Error())
	}

	data := utils.MapToStruct[ChatResponse](res)
	if data == nil {
		return "", fmt.Errorf("openai error: cannot parse response")
	} else if data.Error.Message != "" {
		return "", fmt.Errorf("openai error: %s", data.Error.Message)
	}
	return utils.StoreImagesInMarkdown(data.Choices[0].Message.Content, props.Proxy), nil
}

func hideRequestId(message string) string {
	// xxx (request id: 2024020311120561344953f0xfh0TX)

	exp := regexp.MustCompile(`\(request id: [a-zA-Z0-9]+\)`)
	return exp.ReplaceAllString(message, "")
}

// CreateStreamChatRequest is the stream response body for openai
func (c *ChatInstance) CreateStreamChatRequest(props *adaptercommon.ChatProps, callback globals.Hook) error {
	if globals.IsGeminiImageModel(props.Model) {
		if err := callback(&globals.Chunk{
			Content: "正在生成图片，请稍候...\n\n",
		}); err != nil {
			return err
		}
		if url, err := palm2.NewChatInstance(c.GetEndpoint(), c.GetApiKey()).CreateImage(props); err != nil {
			return err
		} else {
			return callback(&globals.Chunk{
				Content: url,
			})
		}
	}

	if globals.IsOpenAIImageGenerationModel(props.OriginalModel, props.Model) {
		if err := callback(&globals.Chunk{
			Content: "正在生成图片，请稍候...\n\n",
		}); err != nil {
			return err
		}
		if url, err := c.CreateImage(props); err != nil {
			return err
		} else {
			return callback(&globals.Chunk{
				Content: url,
			})
		}
	}

	isCompletionType := props.Model == globals.GPT3TurboInstruct

	ticks := 0
	err := utils.EventScanner(&utils.EventScannerProps{
		Method:  "POST",
		Uri:     c.GetChatEndpoint(props),
		Headers: c.GetHeader(),
		Body:    c.GetChatBody(props, true),
		Callback: func(data string) error {
			ticks += 1

			partial, err := c.ProcessLine(data, isCompletionType)
			if err != nil {
				return err
			}
			return callback(partial)
		},
	}, props.Proxy)

	if err != nil {
		if form := processChatErrorResponse(err.Body); form != nil {
			if form.Error.Type == "" && form.Error.Message == "" {
				return errors.New(utils.ToMarkdownCode("json", err.Body))
			}

			msg := fmt.Sprintf("%s (type: %s)", form.Error.Message, form.Error.Type)
			return errors.New(hideRequestId(msg))
		}
		return err.Error
	}

	if ticks == 0 {
		return errors.New("no response")
	}

	return nil
}
