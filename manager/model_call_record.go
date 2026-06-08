package manager

import (
	"chat/auth"
	"chat/globals"
	"chat/utils"
	"database/sql"
	"fmt"
)

func recordModelCall(db *sql.DB, user *auth.User, buffer *utils.Buffer, quota float32) {
	if db == nil || user == nil || buffer == nil {
		return
	}

	inputTokens := buffer.CountInputToken()
	outputTokens := buffer.CountOutputToken(false)
	inputQuota := buffer.GetInputQuota()
	outputQuota := buffer.GetOutputQuota(false)
	userID := user.GetID(db)

	_, err := globals.ExecDb(db, `
		INSERT INTO model_call_record (
		  user_id, username, model, input_tokens, output_tokens, total_tokens,
		  input_quota, output_quota, quota, input, output
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, user.Username, buffer.GetModel(), inputTokens, outputTokens, inputTokens+outputTokens, inputQuota, outputQuota, quota, buffer.GetRecordPrompts(), buffer.GetRecordResponsePrompts())
	if err != nil {
		globals.Warn(fmt.Sprintf("[model-call-record] failed to insert model call record: %s", err.Error()))
	}
}
