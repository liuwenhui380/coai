package utils

import (
	"fmt"
	"strings"
	"testing"
)

const validPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

func mustPNGDataURL(t *testing.T) string {
	t.Helper()

	return fmt.Sprintf("data:image/png;base64,%s", validPNGBase64)
}

func TestNewImageDecodesPNGDataURL(t *testing.T) {
	dataURL := mustPNGDataURL(t)
	img, err := NewImage(dataURL)
	if err != nil {
		t.Fatalf("NewImage returned error: %v", err)
	}
	if img == nil || img.Object == nil {
		t.Fatal("NewImage returned nil image object")
	}
	if got := img.GetType(); got != "image/png" {
		t.Fatalf("image type = %q, want image/png", got)
	}
}

func TestNewImageDecodesPNGDataURLWithWhitespace(t *testing.T) {
	dataURL := mustPNGDataURL(t)
	dataURL = strings.Replace(dataURL, "base64,", "base64,\n\t", 1)
	dataURL = dataURL[:len(dataURL)/2] + "\n" + dataURL[len(dataURL)/2:] + "\r\n"

	img, err := NewImage(dataURL)
	if err != nil {
		t.Fatalf("NewImage returned error: %v", err)
	}
	if img == nil || img.Object == nil {
		t.Fatal("NewImage returned nil image object")
	}
}
