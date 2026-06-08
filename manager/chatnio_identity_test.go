package manager

import (
	"chat/auth"
	"chat/utils"
	"testing"
)

func TestChatnioUpstreamIdentityUsesStableHashOnly(t *testing.T) {
	user := &auth.User{ID: 42, Username: "alice@example.test"}

	upstreamUser, metadata := chatnioUpstreamIdentity(nil, user)
	wantHash := utils.Sha2Encrypt("42:alice@example.test")

	if upstreamUser != "chatnio-"+wantHash[:16] {
		t.Fatalf("upstream user = %q, want chatnio short hash", upstreamUser)
	}
	if metadata["chatnio_user_hash"] != wantHash {
		t.Fatalf("metadata = %#v, want hash %s", metadata, wantHash)
	}
	for key, value := range metadata {
		if value == user.Username || key == "chatnio_username" {
			t.Fatalf("metadata must not expose raw username: %#v", metadata)
		}
	}
}

func TestChatnioUpstreamIdentityFallsBackToUsernameHash(t *testing.T) {
	user := &auth.User{Username: "bob@example.test"}

	upstreamUser, metadata := chatnioUpstreamIdentity(nil, user)
	wantHash := utils.Sha2Encrypt("bob@example.test")

	if upstreamUser != "chatnio-"+wantHash[:16] {
		t.Fatalf("upstream user = %q, want chatnio short hash", upstreamUser)
	}
	if metadata["chatnio_user_hash"] != wantHash {
		t.Fatalf("metadata = %#v, want hash %s", metadata, wantHash)
	}
}

func TestSetChatnioMetadataValue(t *testing.T) {
	metadata := setChatnioMetadataValue(nil, "chat_id", " 12 ")

	if metadata["chat_id"] != "12" {
		t.Fatalf("metadata = %#v, want trimmed chat_id", metadata)
	}
	metadata = setChatnioMetadataValue(metadata, "empty", " ")
	if _, ok := metadata["empty"]; ok {
		t.Fatalf("empty metadata value should be ignored: %#v", metadata)
	}
}
