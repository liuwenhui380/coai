package utils

import (
	"chat/globals"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func withImageStore(t *testing.T) {
	t.Helper()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	oldAcceptImageStore := globals.AcceptImageStore
	oldNotifyURL := globals.NotifyUrl
	oldProxyEnabled := globals.ImageDownloadProxyEnabled
	oldProxy := globals.ImageDownloadProxy
	oldServeStatic := viper.GetBool("serve_static")

	if err = os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	globals.AcceptImageStore = true
	globals.NotifyUrl = "https://chat.example"
	globals.ImageDownloadProxyEnabled = false
	globals.ImageDownloadProxy = ""
	viper.Set("serve_static", false)

	t.Cleanup(func() {
		_ = os.Chdir(oldWd)
		globals.AcceptImageStore = oldAcceptImageStore
		globals.NotifyUrl = oldNotifyURL
		globals.ImageDownloadProxyEnabled = oldProxyEnabled
		globals.ImageDownloadProxy = oldProxy
		viper.Set("serve_static", oldServeStatic)
	})
}

func TestStoreImagePersistsBase64GeneratedImage(t *testing.T) {
	withImageStore(t)

	url := StoreImage("data:image/png;base64,aGVsbG8=")

	if !strings.HasPrefix(url, "https://chat.example/attachments/") {
		t.Fatalf("stored url = %q, want attachment url", url)
	}

	hash := strings.TrimPrefix(url, "https://chat.example/attachments/")
	data, err := os.ReadFile("storage/attachments/" + hash)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("stored data = %q, want hello", data)
	}
}

func TestStoreImagePersistsDownloadedImage(t *testing.T) {
	withImageStore(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("image-bytes"))
	}))
	defer server.Close()

	url := StoreImage(server.URL + "/generated.png")

	if !strings.HasPrefix(url, "https://chat.example/attachments/") {
		t.Fatalf("stored url = %q, want attachment url", url)
	}
	if !strings.HasSuffix(url, ".png") {
		t.Fatalf("stored url = %q, want .png suffix", url)
	}
}

func TestStoreImagesInMarkdownPersistsRemoteImages(t *testing.T) {
	withImageStore(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("image-bytes"))
	}))
	defer server.Close()

	content := StoreImagesInMarkdown("done ![image](" + server.URL + "/generated.png)")

	if strings.Contains(content, server.URL) {
		t.Fatalf("content still contains remote image url: %s", content)
	}
	if !strings.Contains(content, "https://chat.example/attachments/") {
		t.Fatalf("content = %q, want cached attachment url", content)
	}
}

func TestAttachmentURLUsesAPIPrefixWhenServingStatic(t *testing.T) {
	withImageStore(t)
	viper.Set("serve_static", true)

	url := StoreImage("data:image/png;base64,aGVsbG8=")

	if !strings.HasPrefix(url, "https://chat.example/api/attachments/") {
		t.Fatalf("stored url = %q, want api attachment url", url)
	}
}

func TestImageDownloadProxyConfigUsesSystemProxy(t *testing.T) {
	oldProxyEnabled := globals.ImageDownloadProxyEnabled
	oldProxy := globals.ImageDownloadProxy
	t.Cleanup(func() {
		globals.ImageDownloadProxyEnabled = oldProxyEnabled
		globals.ImageDownloadProxy = oldProxy
	})

	globals.ImageDownloadProxyEnabled = true
	globals.ImageDownloadProxy = "socks5://127.0.0.1:7890"

	config := imageDownloadProxyConfig(nil)

	if len(config) != 1 {
		t.Fatalf("proxy config len = %d, want 1", len(config))
	}
	if config[0].ProxyType != globals.Socks5ProxyType {
		t.Fatalf("proxy type = %d, want socks5", config[0].ProxyType)
	}
	if config[0].Proxy != "socks5://127.0.0.1:7890" {
		t.Fatalf("proxy = %q, want configured proxy", config[0].Proxy)
	}
}
