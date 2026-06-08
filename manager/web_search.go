package manager

import (
	"chat/channel"
	"chat/globals"
	"chat/utils"
)

func shouldSkipWebSearch(model string, messages []globals.Message) bool {
	if globals.IsOpenAIImageGenerationModel(model) || globals.IsGoogleImagenModel(model) {
		return true
	}
	if channel.ConduitInstance != nil && channel.ConduitInstance.IsImageInputModel(model) && messagesContainImages(messages) {
		return true
	}
	return messagesContainImages(messages)
}

func messagesContainImages(messages []globals.Message) bool {
	for _, message := range messages {
		_, images := utils.ExtractImages(message.Content, true)
		if len(images) > 0 {
			return true
		}
	}
	return false
}
