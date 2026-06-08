package utils

import (
	"chat/globals"
	"fmt"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

const base64ImageTokenEstimate = 1000

//   Using https://github.com/pkoukk/tiktoken-go
//   To count number of tokens of openai chat messages
//   OpenAI Cookbook: https://github.com/openai/openai-cookbook/blob/main/examples/How_to_count_tokens_with_tiktoken.ipynb

func GetWeightByModel(model string) int {
	switch model {
	case globals.GPT3TurboInstruct,
		globals.Claude1, globals.Claude1100k,
		globals.Claude2, globals.Claude2100k, globals.Claude2200k:
		return 2
	case globals.GPT3Turbo, globals.GPT3Turbo0613, globals.GPT3Turbo1106, globals.GPT3Turbo0125,
		globals.GPT3Turbo16k, globals.GPT3Turbo16k0613,
		globals.GPT4, globals.GPT40314, globals.GPT40613,
		globals.GPT41106Preview, globals.GPT4TurboPreview, globals.GPT40125Preview,
		globals.GPT4VisionPreview, globals.GPT41106VisionPreview,
		globals.GPT432k, globals.GPT432k0613, globals.GPT432k0314:
		return 3
	case globals.GPT3Turbo0301, globals.GPT3Turbo16k0301:
		return 4 // every message follows <|start|>{role/name}\n{content}<|end|>\n
	default:
		if strings.Contains(model, globals.GPT3Turbo) {
			// warning: gpt-3.5-turbo may update over time. Returning num tokens assuming gpt-3.5-turbo-0613.
			return GetWeightByModel(globals.GPT3Turbo0613)
		} else if strings.Contains(model, globals.GPT4) {
			// warning: gpt-4 may update over time. Returning num tokens assuming gpt-4-0613.
			return GetWeightByModel(globals.GPT40613)
		} else if strings.Contains(model, globals.Claude1) {
			// warning: claude-1 may update over time. Returning num tokens assuming claude-1-100k.
			return GetWeightByModel(globals.Claude1100k)
		} else if strings.Contains(model, globals.Claude2) {
			// warning: claude-2 may update over time. Returning num tokens assuming claude-2-100k.
			return GetWeightByModel(globals.Claude2100k)
		} else {
			// not implemented: See https://github.com/openai/openai-python/blob/main/chatml.md for information on how messages are converted to tokens
			return 3
		}
	}
}
func NumTokensFromMessages(messages []globals.Message, model string, responseType bool) (tokens int) {
	tokensPerMessage := GetWeightByModel(model)
	tkm, err := tiktoken.EncodingForModel(model)

	if err != nil {
		// the method above was deprecated, use the recall method instead
		// can not encode messages, use length of messages as a proxy for number of tokens
		// using rune instead of byte to account for unicode characters (e.g. emojis, non-english characters)
		// data := Marshal(messages)
		// return len([]rune(data)) * weight

		// use the recall method instead (default encoder model is gpt-3.5-turbo-0613)
		if globals.DebugMode {
			globals.Debug(fmt.Sprintf("[tiktoken] error encoding messages: %s (model: %s), using default model instead", err, model))
		}
		return NumTokensFromMessages(messages, globals.GPT3Turbo0613, responseType)
	}

	for _, message := range messages {
		tokens += len(tkm.Encode(message.Content, nil, nil))

		if !responseType {
			tokens += len(tkm.Encode(message.Role, nil, nil)) + tokensPerMessage
		}
	}

	if !responseType {
		tokens += 3 // every reply is primed with <|start|>assistant<|message|>
	}

	if globals.DebugMode {
		globals.Debug(fmt.Sprintf("[tiktoken] num tokens from messages: %d (tokens per message: %d, model: %s)", tokens, tokensPerMessage, model))
	}
	return tokens
}

func NumTokensFromResponse(response string, model string) int {
	if len(response) == 0 {
		return 0
	}

	return NumTokensFromMessages([]globals.Message{{Content: response}}, model, true)
}

func CountInputQuota(charge Charge, token int) float32 {
	if charge.GetType() == globals.TokenBilling {
		return float32(token) / 1000 * charge.GetInput()
	}

	return 0
}

func CountOutputToken(charge Charge, token int) float32 {
	switch charge.GetType() {
	case globals.TokenBilling:
		return float32(token) / 1000 * charge.GetOutput()
	case globals.TimesBilling:
		return charge.GetOutput()
	default:
		return 0
	}
}

// TrimMessagesByTokenLimit trims messages to fit within token and count limits.
// maxTokens: maximum total tokens allowed (e.g., 64000 for 64k)
// maxMessages: maximum number of messages allowed (e.g., 10)
// Returns trimmed messages from the end, ensuring limits are respected
func TrimMessagesByTokenLimit(messages []globals.Message, model string, maxTokens int, maxMessages int) []globals.Message {
	if len(messages) == 0 {
		return messages
	}

	// First, limit by message count
	startIdx := 0
	if len(messages) > maxMessages {
		startIdx = len(messages) - maxMessages
	}

	trimmed := append([]globals.Message(nil), messages[startIdx:]...)

	// Then, limit by token count
	// Calculate tokens from the end, removing oldest messages if needed
	for len(trimmed) > 0 {
		tokens := numTokensFromMessagesForTokenLimit(trimmed, model, false)
		if tokens <= maxTokens {
			break
		}

		if len(trimmed) == 1 {
			trimmed[0] = truncateMessageByTokenLimit(trimmed[0], model, maxTokens)
			break
		}

		// Remove the oldest message and try again
		trimmed = trimmed[1:]
	}

	return trimmed
}

func truncateMessageByTokenLimit(message globals.Message, model string, maxTokens int) globals.Message {
	if numTokensFromMessagesForTokenLimit([]globals.Message{message}, model, false) <= maxTokens {
		return message
	}

	images := ExtractBase64Images(message.Content)
	runes := []rune(message.Content)
	if len(images) > 0 {
		runes = []rune(nonImageTextForTokenLimit(message.Content, images))
	}

	low, high := 0, len(runes)
	best := ""
	if len(images) > 0 {
		best = preserveBase64ImagesWithTextLimit(message.Content, images, 0)
	}

	for low <= high {
		mid := (low + high) / 2
		candidate := message
		if len(images) > 0 {
			candidate.Content = preserveBase64ImagesWithTextLimit(message.Content, images, mid)
		} else {
			candidate.Content = string(runes[:mid])
		}

		if numTokensFromMessagesForTokenLimit([]globals.Message{candidate}, model, false) <= maxTokens {
			best = candidate.Content
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	message.Content = best
	return message
}

func numTokensFromMessagesForTokenLimit(messages []globals.Message, model string, responseType bool) int {
	if len(messages) == 0 {
		return 0
	}

	imageTokens := 0
	sanitized := append([]globals.Message(nil), messages...)
	for idx, message := range sanitized {
		images := ExtractBase64Images(message.Content)
		if len(images) == 0 {
			continue
		}

		imageTokens += len(images) * base64ImageTokenEstimate
		sanitized[idx].Content = removeBase64ImagesForTokenLimit(message.Content, images)
	}

	return NumTokensFromMessages(sanitized, model, responseType) + imageTokens
}

func removeBase64ImagesForTokenLimit(content string, images []string) string {
	for _, image := range images {
		content = strings.ReplaceAll(content, image, "")
	}
	return content
}

func nonImageTextForTokenLimit(content string, images []string) string {
	return removeBase64ImagesForTokenLimit(content, images)
}

func preserveBase64ImagesWithTextLimit(content string, images []string, maxTextRunes int) string {
	var result strings.Builder
	remaining := maxTextRunes
	rest := content

	for _, image := range images {
		index := strings.Index(rest, image)
		if index < 0 {
			continue
		}

		result.WriteString(takeRunes(rest[:index], &remaining))
		result.WriteString(image)
		rest = rest[index+len(image):]
	}

	result.WriteString(takeRunes(rest, &remaining))
	return result.String()
}

func takeRunes(content string, remaining *int) string {
	if *remaining <= 0 || content == "" {
		return ""
	}

	runes := []rune(content)
	if len(runes) <= *remaining {
		*remaining -= len(runes)
		return content
	}

	part := string(runes[:*remaining])
	*remaining = 0
	return part
}
