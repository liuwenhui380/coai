package adaptercommon

import (
	"chat/globals"
	"chat/utils"
)

type RequestProps struct {
	MaxRetries *int                `json:"-"`
	Current    int                 `json:"-"`
	Group      string              `json:"-"`
	Proxy      globals.ProxyConfig `json:"-"`
}

type ChatProps struct {
	RequestProps

	Model         string `json:"model,omitempty"`
	OriginalModel string `json:"-"`

	Message           []globals.Message      `json:"messages,omitempty"`
	MaxTokens         *int                   `json:"max_tokens,omitempty"`
	PresencePenalty   *float32               `json:"presence_penalty,omitempty"`
	FrequencyPenalty  *float32               `json:"frequency_penalty,omitempty"`
	RepetitionPenalty *float32               `json:"repetition_penalty,omitempty"`
	Temperature       *float32               `json:"temperature,omitempty"`
	TopP              *float32               `json:"top_p,omitempty"`
	TopK              *int                   `json:"top_k,omitempty"`
	Tools             *globals.FunctionTools `json:"tools,omitempty"`
	ToolChoice        *interface{}           `json:"tool_choice,omitempty"`
	User              string                 `json:"user,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	Buffer            *utils.Buffer          `json:"-"`
}

func (c *ChatProps) SetupBuffer(buf *utils.Buffer) {
	buf.SetPrompts(c)
	c.Buffer = buf
}

func CreateChatProps(props *ChatProps, buffer *utils.Buffer) *ChatProps {
	// 采样参数在不同上游间约束差异很大，ChatNio 统一交由模型使用服务端默认值。
	props.Temperature = nil
	props.TopP = nil
	props.TopK = nil
	props.PresencePenalty = nil
	props.FrequencyPenalty = nil
	props.RepetitionPenalty = nil
	props.SetupBuffer(buffer)
	return props
}
