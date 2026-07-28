package manager

import (
	adaptercommon "chat/adapter/common"
	"chat/addition/web"
	"chat/admin"
	"chat/auth"
	"chat/channel"
	"chat/globals"
	"chat/utils"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	ReasonStop      = "stop"
	ReasonToolCalls = "tool_calls"
)

func supportRelayPlan() bool {
	return channel.SystemInstance.SupportRelayPlan()
}

func checkChatEnableState(db *sql.DB, cache *redis.Client, user *auth.User, model string, messages []globals.Message) (state error, plan bool) {
	if err := auth.CheckMemberWeeklyLimit(db, user, model); err != nil {
		return err, false
	}

	if supportRelayPlan() {
		return auth.CanEnableModelWithSubscription(db, cache, user, model, messages)
	}

	return auth.CanEnableModel(db, user, model, messages), false
}

func ChatRelayAPI(c *gin.Context) {
	if globals.CloseRelay {
		abortWithErrorResponse(c, fmt.Errorf("relay api is denied of access"), "access_denied_error")
		return
	}

	username := utils.GetUserFromContext(c)
	if username == "" {
		abortWithErrorResponse(c, fmt.Errorf("access denied for invalid api key"), "authentication_error")
		return
	}

	if utils.GetAgentFromContext(c) != "api" {
		abortWithErrorResponse(c, fmt.Errorf("access denied for invalid agent"), "authentication_error")
		return
	}

	var form RelayForm
	if err := c.ShouldBindJSON(&form); err != nil {
		abortWithErrorResponse(c, fmt.Errorf("invalid request body: %s", err.Error()), "invalid_request_error")
		return
	}

	db := utils.GetDBFromContext(c)
	cache := utils.GetCacheFromContext(c)
	user := &auth.User{
		Username: username,
	}
	id := utils.Md5Encrypt(username + form.Model + time.Now().String())
	created := time.Now().Unix()

	messages := transform(form.Messages)
	if strings.HasPrefix(form.Model, "web-") {
		suffix := strings.TrimPrefix(form.Model, "web-")

		form.Model = suffix
		if !shouldSkipWebSearch(form.Model, messages) {
			messages = web.ToSearched(true, messages)
		}
	}

	if strings.HasSuffix(form.Model, "-official") {
		form.Model = strings.TrimSuffix(form.Model, "-official")
		form.Official = true
	}

	check, plan := checkChatEnableState(db, cache, user, form.Model, messages)
	if check != nil {
		sendErrorResponse(c, check, "quota_exceeded_error")
		return
	}

	if form.Stream {
		sendStreamTranshipmentResponse(c, form, messages, id, created, user, plan)
	} else {
		sendTranshipmentResponse(c, form, messages, id, created, user, plan)
	}
}

func getChatProps(form RelayForm, messages []globals.Message, buffer *utils.Buffer, db *sql.DB, user *auth.User) *adaptercommon.ChatProps {
	upstreamUser, metadata := chatnioUpstreamIdentity(db, user)
	return adaptercommon.CreateChatProps(&adaptercommon.ChatProps{
		Model:      form.Model,
		Message:    messages,
		MaxTokens:  form.MaxTokens,
		Tools:      form.Tools,
		ToolChoice: form.ToolChoice,
		User:       upstreamUser,
		Metadata:   metadata,
	}, buffer)
}

func chatnioUpstreamIdentity(db *sql.DB, user *auth.User) (string, map[string]interface{}) {
	if user == nil {
		return "", nil
	}

	parts := make([]string, 0, 2)
	userID := user.HitID()
	if userID <= 0 && db != nil {
		userID = user.GetID(db)
	}
	if userID > 0 {
		parts = append(parts, fmt.Sprintf("%d", userID))
	}
	if username := strings.TrimSpace(user.Username); username != "" {
		parts = append(parts, username)
	}
	if len(parts) == 0 {
		return "", nil
	}

	hash := utils.Sha2Encrypt(strings.Join(parts, ":"))
	return "chatnio-" + hash[:16], map[string]interface{}{
		"chatnio_user_hash": hash,
	}
}

func setChatnioMetadataValue(metadata map[string]interface{}, key, value string) map[string]interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return metadata
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata[key] = value
	return metadata
}

func sendTranshipmentResponse(c *gin.Context, form RelayForm, messages []globals.Message, id string, created int64, user *auth.User, plan bool) {
	db := utils.GetDBFromContext(c)
	cache := utils.GetCacheFromContext(c)

	buffer := utils.NewBuffer(form.Model, messages, channel.ChargeInstance.GetCharge(form.Model))
	hit, err := channel.NewChatRequestWithCache(cache, buffer, auth.GetGroup(db, user), getChatProps(form, messages, buffer, db, user), func(data *globals.Chunk) error {
		buffer.WriteChunk(data)
		return nil
	})

	admin.AnalyseRequest(form.Model, buffer, err)
	if err != nil {
		auth.RevertSubscriptionUsage(db, cache, user, form.Model)
		globals.Warn(fmt.Sprintf("error from chat request api: %s (instance: %s, client: %s)", err, form.Model, c.ClientIP()))

		sendErrorResponse(c, err)
		return
	}

	if !hit {
		CollectQuota(c, user, buffer, plan, err)
	}

	tools := buffer.GetToolCalls()

	c.JSON(http.StatusOK, RelayResponse{
		Id:      fmt.Sprintf("chatcmpl-%s", id),
		Object:  "chat.completion",
		Created: created,
		Model:   form.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: globals.Message{
					Role:         globals.Assistant,
					Content:      buffer.Read(),
					ToolCalls:    tools,
					FunctionCall: buffer.GetFunctionCall(),
				},
				FinishReason: utils.Multi(tools != nil, ReasonToolCalls, ReasonStop),
			},
		},
		Usage: Usage{
			PromptTokens:     buffer.CountInputToken(),
			CompletionTokens: buffer.CountOutputToken(false),
			TotalTokens:      buffer.CountInputToken() + buffer.CountOutputToken(false),
		},
		Quota: utils.Multi[*float32](form.Official, nil, utils.ToPtr(buffer.GetRecordQuota())),
	})
}

func getFinishReason(buffer *utils.Buffer, end bool) interface{} {
	if !end {
		return nil
	}

	if buffer.IsFunctionCalling() {
		return ReasonToolCalls
	}

	return ReasonStop
}

func getRole(data *globals.Chunk) string {
	if data.Content != "" {
		return globals.Assistant
	} else if data.ToolCall != nil {
		return globals.Tool
	} else if data.FunctionCall != nil {
		return globals.Function
	}

	return ""
}

func getStreamTranshipmentForm(id string, created int64, form RelayForm, data *globals.Chunk, buffer *utils.Buffer, end bool, err error) RelayStreamResponse {
	outputTokens := buffer.CountOutputToken(!end)
	quota := buffer.GetQuota()
	if end {
		quota = buffer.GetRecordQuota()
	}

	return RelayStreamResponse{
		Id:      fmt.Sprintf("chatcmpl-%s", id),
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   form.Model,
		Choices: []ChoiceDelta{
			{
				Index: 0,
				Delta: Message{
					Role:         getRole(data),
					Content:      data.Content,
					ToolCalls:    data.ToolCall,
					FunctionCall: data.FunctionCall,
				},
				FinishReason: getFinishReason(buffer, end),
			},
		},
		Usage: Usage{
			PromptTokens:     buffer.CountInputToken(),
			CompletionTokens: outputTokens,
			TotalTokens:      buffer.CountInputToken() + outputTokens,
		},
		Quota: utils.Multi[*float32](form.Official, nil, utils.ToPtr(quota)),
		Error: err,
	}
}

func sendStreamTranshipmentResponse(c *gin.Context, form RelayForm, messages []globals.Message, id string, created int64, user *auth.User, plan bool) {
	partial := make(chan RelayStreamResponse)
	db := utils.GetDBFromContext(c)
	cache := utils.GetCacheFromContext(c)

	group := auth.GetGroup(db, user)
	charge := channel.ChargeInstance.GetCharge(form.Model)

	go func() {
		buffer := utils.NewBuffer(form.Model, messages, charge)
		hit, err := channel.NewChatRequestWithCache(
			cache, buffer, group, getChatProps(form, messages, buffer, db, user),
			func(data *globals.Chunk) error {
				buffer.WriteChunk(data)

				if !data.IsEmpty() {
					partial <- getStreamTranshipmentForm(id, created, form, data, buffer, false, nil)
				}
				return nil
			},
		)

		admin.AnalyseRequest(form.Model, buffer, err)
		if err != nil {
			auth.RevertSubscriptionUsage(db, cache, user, form.Model)
			globals.Warn(fmt.Sprintf("error from chat request api: %s (instance: %s, client: %s)", err.Error(), form.Model, c.ClientIP()))
			partial <- getStreamTranshipmentForm(id, created, form, &globals.Chunk{Content: err.Error()}, buffer, true, err)
			close(partial)
			return
		}

		partial <- getStreamTranshipmentForm(id, created, form, &globals.Chunk{Content: ""}, buffer, true, nil)

		if !hit {
			CollectQuota(c, user, buffer, plan, err)
		}

		close(partial)
		return
	}()

	c.Stream(func(w io.Writer) bool {
		if resp, ok := <-partial; ok {
			if resp.Error != nil {
				sendErrorResponse(c, resp.Error)
				return false
			}

			c.Render(-1, utils.NewEvent(resp))
			return true
		}

		c.Render(-1, utils.NewEndEvent())
		return false
	})
}
