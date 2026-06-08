package openai

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateGPTImage2RequestOmitsUnsupportedSizeAndN(t *testing.T) {
	var request map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aGVsbG8="}]}`))
	}))
	defer server.Close()

	instance := NewChatInstance(server.URL, "test-key")
	url, b64, err := instance.CreateImageRequest(ImageProps{
		Model:  globals.GPTImage2,
		Prompt: "draw a square",
	})
	if err != nil {
		t.Fatal(err)
	}
	if url != "" || b64 != "aGVsbG8=" {
		t.Fatalf("unexpected response url=%q b64=%q", url, b64)
	}
	if _, ok := request["size"]; ok {
		t.Fatalf("gpt-image-2 request should omit size, got %#v", request["size"])
	}
	if _, ok := request["n"]; ok {
		t.Fatalf("gpt-image-2 request should omit n, got %#v", request["n"])
	}
}

func TestCreateImageRequestIncludesRoutingUserAndMetadata(t *testing.T) {
	var request ImageRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aGVsbG8="}]}`))
	}))
	defer server.Close()

	instance := NewChatInstance(server.URL, "test-key")
	_, _, err := instance.CreateImageRequest(ImageProps{
		Model:  globals.GPTImage2,
		Prompt: "draw a square",
		User:   "chatnio-1234567890abcdef",
		Metadata: map[string]interface{}{
			"chatnio_user_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"chat_id":           "42",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if request.User != "chatnio-1234567890abcdef" {
		t.Fatalf("user = %q", request.User)
	}
	if request.Metadata["chatnio_user_hash"] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || request.Metadata["chat_id"] != "42" {
		t.Fatalf("metadata = %#v", request.Metadata)
	}
}

func TestCreateGrokImagineRequestOmitsUnsupportedSizeAndN(t *testing.T) {
	var request map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"data":[{"url":"https://img.example/generated.png"}]}`))
	}))
	defer server.Close()

	instance := NewChatInstance(server.URL, "test-key")
	url, b64, err := instance.CreateImageRequest(ImageProps{
		Model:  "x-ai/grok-imagine-image",
		Prompt: "draw a square",
	})
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://img.example/generated.png" || b64 != "" {
		t.Fatalf("unexpected response url=%q b64=%q", url, b64)
	}
	if _, ok := request["size"]; ok {
		t.Fatalf("grok imagine request should omit size, got %#v", request["size"])
	}
	if _, ok := request["n"]; ok {
		t.Fatalf("grok imagine request should omit n, got %#v", request["n"])
	}
}

func TestCreateImageUsesEditsWhenPromptContainsInputImage(t *testing.T) {
	const dataURL = "data:image/png;base64,iVBORw0KGgo="
	var contentType string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		path = r.URL.Path
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("model") != globals.GPTImage2 {
			t.Fatalf("unexpected model: %s", r.FormValue("model"))
		}
		if strings.Contains(r.FormValue("prompt"), "data:image") {
			t.Fatalf("prompt should be cleaned before edit request: %s", r.FormValue("prompt"))
		}
		_, imageHeader, err := r.FormFile("image")
		if err != nil {
			t.Fatalf("image file is required for edits: %v", err)
		}
		if got := imageHeader.Header.Get("Content-Type"); got != "image/png" {
			t.Fatalf("image multipart content-type = %q, want image/png", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aGVsbG8="}]}`))
	}))
	defer server.Close()

	instance := NewChatInstance(server.URL, "test-key")
	_, err := instance.CreateImage(&adaptercommon.ChatProps{
		Model: globals.GPTImage2,
		Message: []globals.Message{{
			Role:    globals.User,
			Content: "make it brighter " + dataURL,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/v1/images/edits" {
		t.Fatalf("input image should use edits endpoint, got %s", path)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Fatalf("edits endpoint should use multipart/form-data, got %s", contentType)
	}
}
