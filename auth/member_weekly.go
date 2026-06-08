package auth

import (
	"chat/globals"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	MemberTypeStudent = "student"
	MemberTypeTeacher = "teacher"

	ModelChatGPTPro          = "chatgpt-pro"
	ModelChatGPTDeepResearch = "chatgpt-deepresearch"
)

func NormalizeMemberType(memberType string) string {
	switch strings.ToLower(strings.TrimSpace(memberType)) {
	case MemberTypeTeacher:
		return MemberTypeTeacher
	default:
		return MemberTypeStudent
	}
}

func (u *User) GetMemberType(db *sql.DB) string {
	if u == nil {
		return MemberTypeStudent
	}

	if u.MemberType != "" {
		return NormalizeMemberType(u.MemberType)
	}

	var memberType sql.NullString
	if err := globals.QueryRowDb(db, "SELECT member_type FROM auth WHERE id = ?", u.GetID(db)).Scan(&memberType); err != nil {
		return MemberTypeStudent
	}

	u.MemberType = NormalizeMemberType(memberType.String)
	return u.MemberType
}

func WeeklyMemberLimit(memberType, model string) (int, bool) {
	switch NormalizeMemberType(memberType) {
	case MemberTypeTeacher:
		switch model {
		case ModelChatGPTPro:
			return 10, true
		case ModelChatGPTDeepResearch:
			return 5, true
		}
	default:
		switch model {
		case ModelChatGPTPro:
			return 5, true
		case ModelChatGPTDeepResearch:
			return 2, true
		}
	}

	return 0, false
}

func CurrentWeekStart(now time.Time) string {
	local := now.Local()
	offset := (int(local.Weekday()) + 6) % 7
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location()).AddDate(0, 0, -offset)
	return start.Format("2006-01-02")
}

func GetMemberWeeklyUsage(db *sql.DB, user *User, model string, now time.Time) int {
	if db == nil || user == nil {
		return 0
	}

	var used sql.NullInt64
	err := globals.QueryRowDb(db, `
		SELECT used FROM member_weekly_usage
		WHERE user_id = ? AND model = ? AND week_start = ?
	`, user.GetID(db), model, CurrentWeekStart(now)).Scan(&used)
	if err != nil || !used.Valid {
		return 0
	}

	return int(used.Int64)
}

func CheckMemberWeeklyLimit(db *sql.DB, user *User, model string) error {
	if db == nil || user == nil || user.IsAdmin(db) {
		return nil
	}

	memberType := user.GetMemberType(db)
	limit, limited := WeeklyMemberLimit(memberType, model)
	if !limited {
		return nil
	}

	used := GetMemberWeeklyUsage(db, user, model, time.Now())
	if used >= limit {
		return fmt.Errorf("weekly member limit exceeded (model: %s, member: %s, limit: %d)", model, memberType, limit)
	}

	return nil
}

func IncrementMemberWeeklyUsage(db *sql.DB, user *User, model string) bool {
	if db == nil || user == nil || user.IsAdmin(db) {
		return true
	}

	if _, limited := WeeklyMemberLimit(user.GetMemberType(db), model); !limited {
		return true
	}

	userID := user.GetID(db)
	weekStart := CurrentWeekStart(time.Now())
	result, err := globals.ExecDb(db, `
		UPDATE member_weekly_usage
		SET used = used + 1, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND model = ? AND week_start = ?
	`, userID, model, weekStart)
	if err == nil {
		if rows, rowsErr := result.RowsAffected(); rowsErr == nil && rows > 0 {
			return true
		}
	}

	if _, err = globals.ExecDb(db, `
		INSERT INTO member_weekly_usage (user_id, model, week_start, used)
		VALUES (?, ?, ?, ?)
	`, userID, model, weekStart, 1); err == nil {
		return true
	}

	_, retryErr := globals.ExecDb(db, `
		UPDATE member_weekly_usage
		SET used = used + 1, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND model = ? AND week_start = ?
	`, userID, model, weekStart)
	return retryErr == nil
}
