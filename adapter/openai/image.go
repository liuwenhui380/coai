package openai

import (
	"bytes"
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"chat/utils"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path"
	"strings"
	"time"
)

const imageRequestTimeout = 3 * time.Minute

type ImageProps struct {
	Model    string
	Prompt   string
	Size     ImageSize
	Proxy    globals.ProxyConfig
	User     string
	Metadata map[string]interface{}
}

func (c *ChatInstance) GetImageEndpoint() string {
	return fmt.Sprintf("%s/v1/images/generations", c.GetEndpoint())
}

func (c *ChatInstance) GetImageEditEndpoint() string {
	return fmt.Sprintf("%s/v1/images/edits", c.GetEndpoint())
}

// CreateImageRequest will create a dalle image from prompt, return url of image, base64 data and error
func (c *ChatInstance) CreateImageRequest(props ImageProps) (string, string, error) {
	start := time.Now()
	globals.Info(fmt.Sprintf("[openai-image] start generation request (model: %s)", props.Model))
	body := ImageRequest{
		Model:    props.Model,
		Prompt:   props.Prompt,
		User:     props.User,
		Metadata: props.Metadata,
	}
	if !globals.IsOpenAIGPTImageModel(props.Model) {
		body.Size = utils.Multi[ImageSize](
			props.Model == globals.Dalle3 || props.Model == globals.GPTImage1,
			ImageSize1024,
			ImageSize512,
		)
		body.N = 1
	}

	res, err := utils.PostWithTimeout(
		c.GetImageEndpoint(),
		c.GetHeader(),
		body,
		imageRequestTimeout,
		props.Proxy,
	)
	if err != nil || res == nil {
		globals.Warn(fmt.Sprintf("[openai-image] generation request failed after %s (model: %s): %v", time.Since(start).Round(time.Second), props.Model, err))
		return "", "", fmt.Errorf(err.Error())
	}

	data := utils.MapToStruct[ImageResponse](res)
	if data == nil {
		return "", "", fmt.Errorf("openai error: cannot parse response")
	} else if data.Error.Message != "" {
		return "", "", fmt.Errorf(data.Error.Message)
	}

	if len(data.Data) == 0 {
		return "", "", fmt.Errorf("openai error: empty image response")
	}
	if data.Data[0].B64Json != "" {
		globals.Info(fmt.Sprintf("[openai-image] generation request completed after %s (model: %s, output: b64_json)", time.Since(start).Round(time.Second), props.Model))
		return "", data.Data[0].B64Json, nil
	}

	globals.Info(fmt.Sprintf("[openai-image] generation request completed after %s (model: %s, output: url)", time.Since(start).Round(time.Second), props.Model))
	return data.Data[0].Url, "", nil
}

func (c *ChatInstance) CreateImageEditRequest(props ImageProps, images []string) (string, string, error) {
	start := time.Now()
	globals.Info(fmt.Sprintf("[openai-image] start edit request (model: %s, images: %d)", props.Model, len(images)))
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", props.Model); err != nil {
		return "", "", err
	}
	if err := writer.WriteField("prompt", props.Prompt); err != nil {
		return "", "", err
	}

	for i, image := range images {
		data, name, mimeType, err := imageBytesForMultipart(image, props.Proxy)
		if err != nil {
			return "", "", err
		}
		part, err := createImageFormFile(writer, "image", fmt.Sprintf("%d-%s", i, name), mimeType)
		if err != nil {
			return "", "", err
		}
		if _, err = part.Write(data); err != nil {
			return "", "", err
		}
	}
	if err := writer.Close(); err != nil {
		return "", "", err
	}

	headers := c.GetHeader()
	headers["Content-Type"] = writer.FormDataContentType()
	var data ImageResponse
	if err := utils.HttpWithTimeout(c.GetImageEditEndpoint(), http.MethodPost, &data, headers, &body, []globals.ProxyConfig{props.Proxy}, imageRequestTimeout); err != nil {
		globals.Warn(fmt.Sprintf("[openai-image] edit request failed after %s (model: %s, images: %d): %s", time.Since(start).Round(time.Second), props.Model, len(images), err.Error()))
		return "", "", err
	}
	if data.Error.Message != "" {
		globals.Warn(fmt.Sprintf("[openai-image] edit request returned error after %s (model: %s, images: %d): %s", time.Since(start).Round(time.Second), props.Model, len(images), data.Error.Message))
		return "", "", fmt.Errorf(data.Error.Message)
	}
	if len(data.Data) == 0 {
		return "", "", fmt.Errorf("openai error: empty image response")
	}
	if data.Data[0].B64Json != "" {
		globals.Info(fmt.Sprintf("[openai-image] edit request completed after %s (model: %s, images: %d, output: b64_json)", time.Since(start).Round(time.Second), props.Model, len(images)))
		return "", data.Data[0].B64Json, nil
	}
	globals.Info(fmt.Sprintf("[openai-image] edit request completed after %s (model: %s, images: %d, output: url)", time.Since(start).Round(time.Second), props.Model, len(images)))
	return data.Data[0].Url, "", nil
}

func createImageFormFile(writer *multipart.Writer, fieldName string, fileName string, mimeType string) (io.Writer, error) {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeMultipartHeader(fieldName), escapeMultipartHeader(fileName)))
	header.Set("Content-Type", mimeType)
	return writer.CreatePart(header)
}

func escapeMultipartHeader(value string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(value)
}

func imageBytesForMultipart(uri string, proxy globals.ProxyConfig) ([]byte, string, string, error) {
	uri = strings.TrimSpace(uri)
	if strings.HasPrefix(uri, "data:image/") {
		header, payload := splitDataURL(uri)
		if payload == "" {
			return nil, "", "", fmt.Errorf("invalid image data url")
		}
		data, err := decodeDataURL(payload)
		if err != nil {
			return nil, "", "", err
		}
		name := "image" + imageExtFromDataHeader(header)
		return data, name, imageMimeForMultipart(data, name), nil
	}

	downloadURL := normalizeAttachmentURL(uri)
	tmp, err := os.CreateTemp("", "chatnio-image-edit-*"+path.Ext(strings.Split(downloadURL, "?")[0]))
	if err != nil {
		return nil, "", "", err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	if err = utils.DownloadImage(downloadURL, tmpPath, proxy); err != nil {
		return nil, "", "", err
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, "", "", err
	}
	name := path.Base(strings.Split(downloadURL, "?")[0])
	if name == "." || name == "/" || name == "" {
		name = "image.png"
	}
	return data, name, imageMimeForMultipart(data, name), nil
}

func imageMimeForMultipart(data []byte, name string) string {
	detected := http.DetectContentType(data)
	switch detected {
	case "image/jpeg", "image/png", "image/webp":
		return detected
	}
	switch mime.TypeByExtension(strings.ToLower(path.Ext(name))) {
	case "image/jpeg":
		return "image/jpeg"
	case "image/png":
		return "image/png"
	case "image/webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

func splitDataURL(dataURL string) (string, string) {
	parts := utils.SafeSplit(dataURL, ",", 2)
	return parts[0], strings.TrimSpace(parts[1])
}

func decodeDataURL(payload string) ([]byte, error) {
	payload = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ':
			return -1
		default:
			return r
		}
	}, payload)
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if data, err := encoding.DecodeString(payload); err == nil {
			return data, nil
		}
	}
	return base64.StdEncoding.DecodeString(payload)
}

func imageExtFromDataHeader(header string) string {
	switch strings.ToLower(strings.TrimPrefix(strings.Split(header, ";")[0], "data:")) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}

func normalizeAttachmentURL(uri string) string {
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") || globals.NotifyUrl == "" {
		return uri
	}
	if strings.HasPrefix(uri, "/api/attachments/") || strings.HasPrefix(uri, "/attachments/") {
		return strings.TrimRight(globals.NotifyUrl, "/") + uri
	}
	return uri
}

// CreateImage will create a dalle image from prompt, return markdown of image
func (c *ChatInstance) CreateImage(props *adaptercommon.ChatProps) (string, error) {
	prompt, images := utils.ExtractImages(c.GetLatestPrompt(props), true)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" && len(images) > 0 {
		prompt = "Generate one image based on the provided image."
	}

	imageProps := ImageProps{
		Model:    props.Model,
		Prompt:   prompt,
		Proxy:    props.Proxy,
		User:     props.User,
		Metadata: props.Metadata,
	}

	var url, b64Json string
	var err error
	if len(images) > 0 {
		url, b64Json, err = c.CreateImageEditRequest(imageProps, images)
	} else {
		url, b64Json, err = c.CreateImageRequest(imageProps)
	}
	if err != nil {
		if strings.Contains(err.Error(), "safety") {
			return err.Error(), nil
		}
		return "", err
	}

	if b64Json != "" {
		storedUrl := utils.StoreImage(fmt.Sprintf("data:image/png;base64,%s", b64Json))
		return utils.GetImageMarkdown(storedUrl), nil
	}

	storedUrl := utils.StoreImageWithProxy(url, props.Proxy)
	return utils.GetImageMarkdown(storedUrl), nil
}
