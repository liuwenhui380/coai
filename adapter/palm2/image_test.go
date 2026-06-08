package palm2

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestCreateGeminiImageRequestUsesGenerateContent(t *testing.T) {
	var request GeminiChatBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-3.1-flash-image-preview:generateContent" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"inline_data":{"mime_type":"image/png","data":"aGVsbG8="}},{"text":"done"}]}}]}`))
	}))
	defer server.Close()

	instance := NewChatInstance(server.URL, "test-key")
	image, err := instance.CreateImageRequest(ImageProps{
		Model:  "gemini-3.1-flash-image-preview",
		Prompt: "A cute cat strolls under cherry blossom trees.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if image != "data:image/png;base64,aGVsbG8=" {
		t.Fatalf("unexpected image response: %q", image)
	}
	if got := request.GenerationConfig.ResponseModalities; len(got) != 2 || got[0] != "IMAGE" || got[1] != "TEXT" {
		t.Fatalf("unexpected response modalities: %#v", got)
	}
	if len(request.Contents) == 0 || len(request.Contents[0].Parts) == 0 || request.Contents[0].Parts[0].Text == nil {
		t.Fatal("expected prompt text in Gemini image request body")
	}
}

func TestCreateImageReturnsCachedMarkdownForGeminiImageModel(t *testing.T) {
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	oldAcceptImageStore := globals.AcceptImageStore
	oldNotifyURL := globals.NotifyUrl
	globals.AcceptImageStore = true
	globals.NotifyUrl = "https://chat.example"
	defer func() {
		globals.AcceptImageStore = oldAcceptImageStore
		globals.NotifyUrl = oldNotifyURL
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"inline_data":{"mime_type":"image/png","data":"aGVsbG8="}}]}}]}`))
	}))
	defer server.Close()

	instance := NewChatInstance(server.URL, "test-key")
	markdown, err := instance.CreateImage(&adaptercommon.ChatProps{
		Model: "gemini-3.1-flash-image-preview",
		Message: []globals.Message{{
			Role:    globals.User,
			Content: "draw a cat",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(markdown, "https://chat.example/attachments/") {
		t.Fatalf("unexpected markdown: %s", markdown)
	}
}
