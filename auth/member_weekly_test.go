package auth

import (
	"chat/globals"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupMemberWeeklyTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		globals.SqliteEngine = false
		_ = db.Close()
	})

	globals.SqliteEngine = true
	for _, stmt := range []string{
		`CREATE TABLE auth (
			id INT PRIMARY KEY AUTO_INCREMENT,
			username VARCHAR(24) UNIQUE,
			token VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE,
			password VARCHAR(64) NOT NULL,
			is_admin BOOLEAN DEFAULT FALSE,
			is_banned BOOLEAN DEFAULT FALSE,
			member_type VARCHAR(32) NOT NULL DEFAULT 'student'
		);`,
		`CREATE TABLE member_weekly_usage (
			id INT PRIMARY KEY AUTO_INCREMENT,
			user_id INT,
			model VARCHAR(255) NOT NULL,
			week_start VARCHAR(10) NOT NULL,
			used INT DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY (user_id, model, week_start)
		);`,
	} {
		if _, err := globals.ExecDb(db, stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}

	return db
}

func insertMemberWeeklyUser(t *testing.T, db *sql.DB, username, memberType string, admin bool) *User {
	t.Helper()

	result, err := globals.ExecDb(db, `
		INSERT INTO auth (username, token, email, password, is_admin, member_type)
		VALUES (?, ?, ?, ?, ?, ?)
	`, username, username+"-token", username+"@example.com", "password", admin, memberType)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}

	return &User{ID: id, Username: username, Admin: admin, MemberType: memberType}
}

func TestNormalizeMemberType(t *testing.T) {
	if got := NormalizeMemberType(""); got != MemberTypeStudent {
		t.Fatalf("empty member type = %q, want student", got)
	}
	if got := NormalizeMemberType("TEACHER"); got != MemberTypeTeacher {
		t.Fatalf("teacher member type = %q, want teacher", got)
	}
	if got := NormalizeMemberType("other"); got != MemberTypeStudent {
		t.Fatalf("invalid member type = %q, want student", got)
	}
}

func TestMemberWeeklyLimit(t *testing.T) {
	tests := []struct {
		memberType string
		model      string
		limit      int
		limited    bool
	}{
		{MemberTypeStudent, ModelChatGPTPro, 5, true},
		{MemberTypeStudent, ModelChatGPTDeepResearch, 2, true},
		{MemberTypeTeacher, ModelChatGPTPro, 10, true},
		{MemberTypeTeacher, ModelChatGPTDeepResearch, 5, true},
		{MemberTypeTeacher, "chatgpt-image", 0, false},
	}

	for _, tt := range tests {
		limit, limited := WeeklyMemberLimit(tt.memberType, tt.model)
		if limit != tt.limit || limited != tt.limited {
			t.Fatalf("WeeklyMemberLimit(%q, %q) = (%d, %v), want (%d, %v)", tt.memberType, tt.model, limit, limited, tt.limit, tt.limited)
		}
	}
}

func TestStudentProWeeklyLimit(t *testing.T) {
	db := setupMemberWeeklyTestDB(t)
	user := insertMemberWeeklyUser(t, db, "student", MemberTypeStudent, false)

	for i := 0; i < 5; i++ {
		if err := CheckMemberWeeklyLimit(db, user, ModelChatGPTPro); err != nil {
			t.Fatalf("check before use %d: %v", i+1, err)
		}
		if !IncrementMemberWeeklyUsage(db, user, ModelChatGPTPro) {
			t.Fatalf("increment use %d failed", i+1)
		}
	}

	if err := CheckMemberWeeklyLimit(db, user, ModelChatGPTPro); err == nil {
		t.Fatal("expected student pro weekly limit error")
	}
}

func TestTeacherDeepResearchWeeklyLimit(t *testing.T) {
	db := setupMemberWeeklyTestDB(t)
	user := insertMemberWeeklyUser(t, db, "teacher", MemberTypeTeacher, false)

	for i := 0; i < 5; i++ {
		if err := CheckMemberWeeklyLimit(db, user, ModelChatGPTDeepResearch); err != nil {
			t.Fatalf("check before use %d: %v", i+1, err)
		}
		if !IncrementMemberWeeklyUsage(db, user, ModelChatGPTDeepResearch) {
			t.Fatalf("increment use %d failed", i+1)
		}
	}

	if err := CheckMemberWeeklyLimit(db, user, ModelChatGPTDeepResearch); err == nil {
		t.Fatal("expected teacher deepresearch weekly limit error")
	}
}

func TestMemberWeeklyLimitAdminBypassAndNonLimitedNoop(t *testing.T) {
	db := setupMemberWeeklyTestDB(t)
	admin := insertMemberWeeklyUser(t, db, "admin", MemberTypeStudent, true)
	student := insertMemberWeeklyUser(t, db, "image-user", MemberTypeStudent, false)

	for i := 0; i < 7; i++ {
		if !IncrementMemberWeeklyUsage(db, admin, ModelChatGPTPro) {
			t.Fatalf("admin increment %d failed", i+1)
		}
	}
	if err := CheckMemberWeeklyLimit(db, admin, ModelChatGPTPro); err != nil {
		t.Fatalf("admin should bypass weekly limit: %v", err)
	}
	if used := GetMemberWeeklyUsage(db, admin, ModelChatGPTPro, time.Now()); used != 0 {
		t.Fatalf("admin usage = %d, want 0 because admin is not counted", used)
	}

	if !IncrementMemberWeeklyUsage(db, student, "chatgpt-image") {
		t.Fatal("non-limited model increment should no-op successfully")
	}
	if err := CheckMemberWeeklyLimit(db, student, "chatgpt-image"); err != nil {
		t.Fatalf("non-limited model should not be blocked: %v", err)
	}
	if used := GetMemberWeeklyUsage(db, student, "chatgpt-image", time.Now()); used != 0 {
		t.Fatalf("non-limited usage = %d, want 0", used)
	}
}
