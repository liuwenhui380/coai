package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func mustPNGDataURL(t *testing.T) string {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	return fmt.Sprintf("data:image/png;base64,%s", Base64EncodeBytes(buf.Bytes()))
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
