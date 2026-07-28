package hunyuan

import (
	"chat/globals"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewRequestOmitsSamplingControls(t *testing.T) {
	request := NewRequest(Stream, []globals.Message{{
		Role:    globals.User,
		Content: "hello",
	}})

	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	client := NewInstance(123, "hunyuan.cloud.tencent.com", NewCredential("secret-id", "secret-key"))
	request.AppID = client.AppID
	request.SecretID = client.Credential.SecretID
	signatureURL := client.buildURL(request)

	for _, field := range []string{"temperature", "top_p"} {
		if strings.Contains(string(raw), field) {
			t.Fatalf("Hunyuan request JSON must not contain %s: %s", field, raw)
		}
		if strings.Contains(signatureURL, field+"=") {
			t.Fatalf("Hunyuan signature URL must not contain %s: %s", field, signatureURL)
		}
	}
}
