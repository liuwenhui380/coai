package utils

import (
	"chat/globals"
	"encoding/base64"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/chai2010/webp"
	"github.com/spf13/viper"
)

type Image struct {
	Object  image.Image
	Content string
}
type Images []Image

func NewImage(url string) (*Image, error) {
	if strings.HasPrefix(url, "data:image/") {
		data := SafeSplit(url, ",", 2)
		if data[1] == "" {
			return nil, nil
		}

		decoded, err := Base64Decode(data[1])
		if err != nil {
			return nil, err
		}

		img, _, err := image.Decode(strings.NewReader(string(decoded)))
		if err != nil {
			return nil, err
		}

		return &Image{Object: img, Content: url}, nil
	}

	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	var img image.Image
	suffix := strings.ToLower(path.Ext(url))
	switch suffix {
	case ".png":
		if img, _, err = image.Decode(res.Body); err != nil {
			return nil, err
		}
	case ".jpg", ".jpeg":
		if img, err = jpeg.Decode(res.Body); err != nil {
			return nil, err
		}
	case "webp":
		if img, err = webp.Decode(res.Body); err != nil {
			return nil, err
		}
	case "gif":
		ticks, err := gif.DecodeAll(res.Body)
		if err != nil {
			return nil, err
		}
		img = ticks.Image[0]
	}

	return &Image{Object: img, Content: url}, nil
}

func NewImageContent(content string) *Image {
	return &Image{Content: content}
}

func ConvertToBase64(url string) (string, error) {
	if strings.HasPrefix(url, "data:image/") {
		data := strings.Split(url, ",")
		if len(data) != 2 {
			return "", nil
		}
		return data[1], nil
	}

	res, err := http.Get(url)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return Base64EncodeBytes(data), nil
}

func (i *Image) GetWidth() int {
	return i.Object.Bounds().Max.X
}

func (i *Image) GetHeight() int {
	return i.Object.Bounds().Max.Y
}

func (i *Image) GetPixel(x int, y int) (uint32, uint32, uint32, uint32) {
	return i.Object.At(x, y).RGBA()
}

func (i *Image) GetPixelColor(x int, y int) (int, int, int) {
	r, g, b, _ := i.GetPixel(x, y)
	return int(r), int(g), int(b)
}

func (i *Image) CountTokens(model string) int {
	if globals.IsVisionModel(model) {
		// tile size is 512x512
		// the max size of image is 2048x2048
		// the image that is larger than 2048x2048 will be resized in 16 tiles

		x := LimitMax(math.Ceil(float64(i.GetWidth())/512), 4)
		y := LimitMax(math.Ceil(float64(i.GetHeight())/512), 4)
		tiles := int(x) * int(y)

		return 85 + 170*tiles
	}

	return 0
}

func (i *Image) IsBase64() bool {
	return strings.HasPrefix(i.Content, "data:image/")
}

func (i *Image) GetType() string {
	// example: image/jpeg, image/png, image/gif

	if i.IsBase64() {
		t := SafeSplit(i.Content, ";", 2)[0]
		return strings.ReplaceAll(t, "data:", "")
	}

	// example: .jpg, .png, .gif to image/jpeg, image/png, image/gif
	switch strings.ToLower(path.Ext(i.Content)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return ""
	}
}

func (i *Image) ToBase64() string {
	if i.IsBase64() {
		return i.Content
	}

	// get url content and convert to base64
	data, err := ConvertToBase64(i.Content)
	if err != nil {
		globals.Warn(fmt.Sprintf("cannot convert image to base64: %s", err.Error()))
		return ""
	}

	return fmt.Sprintf("data:%s;base64,%s", i.GetType(), data)
}

func (i *Image) ToRawBase64() string {
	// example: return /9j/...
	if i.IsBase64() {
		return SafeSplit(i.Content, ",", 2)[1]
	}

	// get url content and convert to base64
	data, err := ConvertToBase64(i.Content)
	if err != nil {
		globals.Warn(fmt.Sprintf("cannot convert image to base64: %s", err.Error()))
		return ""
	}

	return data
}

func DownloadImage(uri string, filePath string, config ...globals.ProxyConfig) error {
	client := newClient(config)
	res, err := client.Get(uri)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			globals.Debug(fmt.Sprintf("[utils] close file error: %s (path: %s)", err.Error(), filePath))
		}
	}(res.Body)

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("download image failed with status code: %d", res.StatusCode)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			globals.Debug(fmt.Sprintf("[utils] close file error: %s (path: %s)", err.Error(), filePath))
		}
	}(file)

	_, err = io.Copy(file, res.Body)
	return err
}

func StoreImage(url string) string {
	return StoreImageWithProxy(url)
}

func StoreImagesInMarkdown(content string, config ...globals.ProxyConfig) string {
	if !globals.AcceptImageStore || content == "" {
		return content
	}

	_, images := ExtractImages(content, true)
	for _, image := range images {
		stored := StoreImageWithProxy(image, config...)
		if stored != "" && stored != image {
			content = strings.ReplaceAll(content, image, stored)
		}
	}

	return content
}

func StoreImageWithProxy(uri string, config ...globals.ProxyConfig) string {
	if globals.AcceptImageStore {
		if strings.HasPrefix(uri, "data:image/") {
			return StoreBase64Image(uri)
		}

		hash := Md5Encrypt(uri) + imageExtensionFromURL(uri)
		filePath := fmt.Sprintf("storage/attachments/%s", hash)

		if err := ensureAttachmentDir(); err != nil {
			globals.Warn(fmt.Sprintf("[utils] create image storage dir error: %s", err.Error()))
			return uri
		}

		if err := DownloadImage(uri, filePath, imageDownloadProxyConfig(config)...); err != nil {
			globals.Warn(fmt.Sprintf("[utils] save image error: %s", err.Error()))
			return uri
		}

		return attachmentURL(hash)
	}

	return uri
}

func StoreBase64Image(dataURL string) string {
	if !globals.AcceptImageStore {
		return dataURL
	}

	header, payload := splitDataImageURI(dataURL)
	if payload == "" {
		globals.Warn("[utils] save base64 image error: invalid data url")
		return dataURL
	}

	data, err := decodeImageBase64(payload)
	if err != nil {
		globals.Warn(fmt.Sprintf("[utils] save base64 image error: %s", err.Error()))
		return dataURL
	}

	hash := Md5Encrypt(dataURL) + imageExtensionFromDataURL(header)
	filePath := fmt.Sprintf("storage/attachments/%s", hash)

	if err = ensureAttachmentDir(); err != nil {
		globals.Warn(fmt.Sprintf("[utils] create image storage dir error: %s", err.Error()))
		return dataURL
	}

	if err = os.WriteFile(filePath, data, 0644); err != nil {
		globals.Warn(fmt.Sprintf("[utils] save base64 image error: %s", err.Error()))
		return dataURL
	}

	return attachmentURL(hash)
}

func ensureAttachmentDir() error {
	return os.MkdirAll("storage/attachments", 0755)
}

func attachmentURL(hash string) string {
	prefix := "/attachments"
	if viper.GetBool("serve_static") {
		prefix = "/api/attachments"
	}

	return fmt.Sprintf("%s%s/%s", strings.TrimRight(globals.NotifyUrl, "/"), prefix, hash)
}

func imageDownloadProxyConfig(config []globals.ProxyConfig) []globals.ProxyConfig {
	if len(config) > 0 && config[0].ProxyType != globals.NoneProxyType && strings.TrimSpace(config[0].Proxy) != "" {
		return config
	}

	proxyURL := strings.TrimSpace(globals.ImageDownloadProxy)
	if !globals.ImageDownloadProxyEnabled || proxyURL == "" {
		return config
	}

	proxyType := globals.HttpProxyType
	if strings.HasPrefix(strings.ToLower(proxyURL), "socks5://") {
		proxyType = globals.Socks5ProxyType
	}

	return []globals.ProxyConfig{{
		ProxyType: proxyType,
		Proxy:     proxyURL,
	}}
}

func imageExtensionFromURL(uri string) string {
	if ext := strings.ToLower(path.Ext(strings.Split(uri, "?")[0])); ext != "" {
		return ext
	}
	return ".png"
}

func imageExtensionFromDataURL(header string) string {
	mimeType := strings.TrimPrefix(strings.Split(header, ";")[0], "data:")
	switch strings.ToLower(mimeType) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	default:
		return ".png"
	}
}

func splitDataImageURI(dataURL string) (string, string) {
	parts := SafeSplit(strings.TrimSpace(dataURL), ",", 2)
	if len(parts) < 2 || parts[1] == "" {
		return dataURL, ""
	}

	return parts[0], strings.TrimSpace(parts[1])
}

func decodeImageBase64(payload string) ([]byte, error) {
	payload = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ':
			return -1
		default:
			return r
		}
	}, payload)

	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		if data, err := encoding.DecodeString(payload); err == nil {
			return data, nil
		}
	}

	return base64.StdEncoding.DecodeString(payload)
}
