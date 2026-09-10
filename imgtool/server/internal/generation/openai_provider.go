package generation

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"net/url"
	"regexp"
	"strings"
	"time"

	"imgtool/server/internal/channels"
)

type OpenAIProvider struct {
	Client          *http.Client
	InternalBaseURL string
}

func (p OpenAIProvider) GenerateText(snapshot channels.Snapshot, request TextRequest) (string, error) {
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	endpoint, err := url.JoinPath(ensureTrailingSlash(p.requestBaseURL(snapshot)), "v1/chat/completions")
	if err != nil {
		return "", err
	}
	body := map[string]any{
		"model": snapshot.ModelID,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": request.Prompt},
					{
						"type": "image_url",
						"image_url": map[string]any{
							"url": dataURL(request.MimeType, request.Image),
						},
					},
				},
			},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	httpRequest, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+snapshot.APIKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := client.Do(httpRequest)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	responseBytes, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("upstream error %d: %s", response.StatusCode, redact(responseBytes, snapshot.APIKey))
	}
	var decoded any
	if err := json.Unmarshal(responseBytes, &decoded); err != nil {
		return "", err
	}
	text := extractText(decoded)
	if strings.TrimSpace(text) == "" {
		return "", errors.New("unsupported text response")
	}
	return text, nil
}

func (p OpenAIProvider) GenerateImage(snapshot channels.Snapshot, request ImageRequest) ([]ImageResult, error) {
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	if isGeminiChatImageModel(snapshot.ModelID) {
		return generateChatImages(client, snapshot, request)
	}
	if isAihubMixDoubaoSeedream(snapshot) {
		return generateAihubMixDoubaoImages(client, snapshot, request)
	}
	isEdit := snapshot.Capability == "image-to-image" || request.Capability == "image-to-image"
	path := "v1/images/generations"
	if isEdit {
		path = "v1/images/edits"
	}
	endpoint, err := url.JoinPath(ensureTrailingSlash(p.requestBaseURL(snapshot)), path)
	if err != nil {
		return nil, err
	}
	count := 1
	request.Count = count
	results, err := p.generateSingleOpenAIImage(client, snapshot, request, endpoint, isEdit)
	if err != nil {
		return nil, rewriteImageUpstreamError(err)
	}
	if len(results) > count {
		results = results[:count]
	}
	if len(results) == 0 {
		return nil, errors.New("unsupported image response")
	}
	return results, nil
}

func (p OpenAIProvider) generateSingleOpenAIImage(client *http.Client, snapshot channels.Snapshot, request ImageRequest, endpoint string, isEdit bool) ([]ImageResult, error) {
	var httpRequest *http.Request
	if isEdit {
		if len(request.Image) == 0 {
			return nil, errors.New("image-to-image requires input image")
		}
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("model", snapshot.ModelID)
		_ = writer.WriteField("prompt", request.Prompt)
		_ = writer.WriteField("n", fmt.Sprint(request.Count))
		applyImageGeometryFields(func(name, value string) {
			_ = writer.WriteField(name, value)
		}, snapshot.ModelID, request, true)
		mimeType := strings.TrimSpace(request.MimeType)
		if mimeType == "" {
			mimeType = http.DetectContentType(request.Image)
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="%s"`, inputImageFilename(mimeType)))
		header.Set("Content-Type", mimeType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(request.Image); err != nil {
			return nil, err
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
		httpRequest, err = http.NewRequest(http.MethodPost, endpoint, body)
		if err != nil {
			return nil, err
		}
		httpRequest.Header.Set("Content-Type", writer.FormDataContentType())
	} else {
		body := map[string]any{
			"model":  snapshot.ModelID,
			"prompt": request.Prompt,
			"n":      request.Count,
		}
		applyImageGeometryFields(func(name, value string) {
			body[name] = value
		}, snapshot.ModelID, request, false)
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		httpRequest, err = http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	httpRequest.Header.Set("Authorization", "Bearer "+snapshot.APIKey)
	requestStarted := time.Now()
	var responseHeaderAt time.Time
	httpRequest = httpRequest.WithContext(httptrace.WithClientTrace(httpRequest.Context(), &httptrace.ClientTrace{
		GotFirstResponseByte: func() {
			if responseHeaderAt.IsZero() {
				responseHeaderAt = time.Now()
			}
		},
	}))
	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if responseHeaderAt.IsZero() {
		responseHeaderAt = time.Now()
	}
	bodyStarted := time.Now()
	trackedBody := &firstByteReader{reader: response.Body}
	responseBytes, err := io.ReadAll(io.LimitReader(trackedBody, 32*1024*1024))
	bodyReadAt := time.Now()
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		err := fmt.Errorf("upstream error %d: %s", response.StatusCode, redact(responseBytes, snapshot.APIKey))
		if shouldFallbackToChatImage(err) {
			return generateChatImages(client, snapshot, request)
		}
		return nil, err
	}
	responseEntries, err := decodeImageResponseEntries(responseBytes, response.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}
	log.Printf(
		"generation stage=upstream_image_response channel=%s model=%s entries=%d duration_ms=%d response_header_ms=%d first_body_byte_ms=%d read_body_ms=%d response_bytes=%d",
		snapshot.ChannelName,
		snapshot.ModelID,
		len(responseEntries),
		time.Since(requestStarted).Milliseconds(),
		responseHeaderAt.Sub(requestStarted).Milliseconds(),
		trackedBody.firstByteDuration(bodyStarted),
		bodyReadAt.Sub(bodyStarted).Milliseconds(),
		len(responseBytes),
	)
	results, err := imageEntriesToResults(client, responseEntries, request.Count)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, errors.New("unsupported image response")
	}
	return results, nil
}

func decodeImageResponseEntries(responseBytes []byte, contentType string) ([]imageEntry, error) {
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return extractImageEntriesFromSSE(responseBytes)
	}
	var decoded any
	if err := json.Unmarshal(responseBytes, &decoded); err != nil {
		return nil, err
	}
	return extractImageEntries(decoded), nil
}

func extractImageEntriesFromSSE(responseBytes []byte) ([]imageEntry, error) {
	normalized := strings.ReplaceAll(string(responseBytes), "\r\n", "\n")
	events := strings.Split(normalized, "\n\n")
	entries := []imageEntry{}
	for _, event := range events {
		dataLines := []string{}
		for _, line := range strings.Split(event, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" || data == "[DONE]" {
				continue
			}
			dataLines = append(dataLines, data)
		}
		if len(dataLines) == 0 {
			continue
		}
		data := strings.Join(dataLines, "\n")
		var decoded any
		if err := json.Unmarshal([]byte(data), &decoded); err != nil {
			collectImageStrings(data, &entries)
			continue
		}
		entries = append(entries, extractImageEntries(decoded)...)
	}
	return entries, nil
}

func generateAihubMixDoubaoImages(client *http.Client, snapshot channels.Snapshot, request ImageRequest) ([]ImageResult, error) {
	count := request.Count
	if count < 1 {
		count = 1
	}
	endpoint, err := url.JoinPath(ensureTrailingSlash(snapshot.BaseURL), "v1/models/doubao", snapshot.ModelID, "predictions")
	if err != nil {
		return nil, err
	}
	input := map[string]any{
		"prompt":                      request.Prompt,
		"size":                        normalizeDoubaoSize(request.Size),
		"n":                           count,
		"sequential_image_generation": "disabled",
		"stream":                      false,
		"response_format":             "url",
		"watermark":                   false,
	}
	if len(request.Image) > 0 {
		input["image"] = dataURL(request.MimeType, request.Image)
	}
	payload, err := json.Marshal(map[string]any{"input": input})
	if err != nil {
		return nil, err
	}
	httpRequest, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+snapshot.APIKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBytes, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream error %d: %s", response.StatusCode, redact(responseBytes, snapshot.APIKey))
	}
	var decoded any
	if err := json.Unmarshal(responseBytes, &decoded); err != nil {
		return nil, err
	}
	results, err := imageEntriesToResults(client, extractImageEntries(decoded), count)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, errors.New("unsupported image response")
	}
	return results, nil
}

func generateChatImages(client *http.Client, snapshot channels.Snapshot, request ImageRequest) ([]ImageResult, error) {
	count := request.Count
	if count < 1 {
		count = 1
	}
	results := []ImageResult{}
	attempts := count
	for attempt := 0; attempt < attempts && len(results) < count; attempt++ {
		entries, err := requestChatImages(client, snapshot, request)
		if err != nil {
			return nil, err
		}
		images, err := imageEntriesToResults(client, entries, count-len(results))
		if err != nil {
			return nil, err
		}
		results = append(results, images...)
	}
	if len(results) == 0 {
		return nil, errors.New("unsupported chat image response")
	}
	return results, nil
}

func requestChatImages(client *http.Client, snapshot channels.Snapshot, request ImageRequest) ([]imageEntry, error) {
	endpoint, err := url.JoinPath(ensureTrailingSlash(snapshot.BaseURL), "v1/chat/completions")
	if err != nil {
		return nil, err
	}
	content := []map[string]any{
		{"type": "text", "text": buildChatImagePrompt(request)},
	}
	if len(request.Image) > 0 {
		content = append(content, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": dataURL(request.MimeType, request.Image),
			},
		})
	}
	body := map[string]any{
		"model": snapshot.ModelID,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpRequest, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+snapshot.APIKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBytes, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream error %d: %s", response.StatusCode, redact(responseBytes, snapshot.APIKey))
	}
	var decoded any
	if err := json.Unmarshal(responseBytes, &decoded); err != nil {
		return nil, err
	}
	return extractImageEntries(decoded), nil
}

func buildChatImagePrompt(request ImageRequest) string {
	parts := []string{request.Prompt}
	if request.Size != "" {
		parts = append(parts, "尺寸/比例要求："+request.Size)
	}
	return strings.Join(parts, "\n")
}

type imageEntry struct {
	Kind     string
	Value    string
	MimeType string
}

type firstByteReader struct {
	reader      io.Reader
	firstByteAt time.Time
}

func (r *firstByteReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 && r.firstByteAt.IsZero() {
		r.firstByteAt = time.Now()
	}
	return n, err
}

func (r *firstByteReader) firstByteDuration(started time.Time) int64 {
	if r.firstByteAt.IsZero() {
		return 0
	}
	return r.firstByteAt.Sub(started).Milliseconds()
}

func imageEntriesToResults(client *http.Client, entries []imageEntry, count int) ([]ImageResult, error) {
	results := []ImageResult{}
	for _, entry := range entries {
		switch entry.Kind {
		case "base64":
			bytes, err := decodeBase64Image(entry.Value)
			if err != nil {
				return nil, err
			}
			mimeType := entry.MimeType
			if mimeType == "" {
				mimeType = "image/png"
			}
			results = append(results, ImageResult{Bytes: bytes, MimeType: mimeType})
		case "url":
			image, err := downloadImage(client, entry.Value)
			if err != nil {
				return nil, err
			}
			results = append(results, image)
		}
		if len(results) >= count {
			break
		}
	}
	return results, nil
}

func downloadImage(client *http.Client, imageURL string) (ImageResult, error) {
	request, err := http.NewRequest(http.MethodGet, imageURL, nil)
	if err != nil {
		return ImageResult{}, err
	}
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return ImageResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ImageResult{}, fmt.Errorf("download image failed: %d", response.StatusCode)
	}
	bytes, err := io.ReadAll(io.LimitReader(response.Body, 25*1024*1024))
	if err != nil {
		return ImageResult{}, err
	}
	mimeType := strings.Split(response.Header.Get("Content-Type"), ";")[0]
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	log.Printf("generation stage=download_image mime=%s bytes=%d duration_ms=%d", mimeType, len(bytes), time.Since(started).Milliseconds())
	return ImageResult{Bytes: bytes, MimeType: mimeType}, nil
}

func extractImageEntries(value any) []imageEntry {
	entries := []imageEntry{}
	collectImageEntries(value, &entries, 0)
	return entries
}

func collectImageEntries(value any, entries *[]imageEntry, depth int) {
	if depth > 64 {
		return
	}
	switch typed := value.(type) {
	case string:
		collectImageStrings(typed, entries)
	case []any:
		for _, item := range typed {
			collectImageEntries(item, entries, depth+1)
		}
	case map[string]any:
		if entry, ok := extractInlineImageEntry(typed); ok {
			pushImageEntry(entries, entry)
		}
		for key, item := range typed {
			switch itemValue := item.(type) {
			case string:
				if key == "b64_json" {
					pushImageEntry(entries, imageEntry{Kind: "base64", Value: itemValue, MimeType: "image/png"})
					continue
				}
				if (key == "url" || key == "image_url") && strings.HasPrefix(strings.ToLower(itemValue), "http") {
					pushImageEntry(entries, imageEntry{Kind: "url", Value: itemValue})
					continue
				}
				if key == "url" || key == "image_url" || key == "image" || key == "b64_json" || key == "content" || key == "text" || key == "data" {
					collectImageStrings(itemValue, entries)
				}
			case []any, map[string]any:
				collectImageEntries(itemValue, entries, depth+1)
			}
		}
	}
}

func collectImageStrings(value string, entries *[]imageEntry) {
	if strings.HasPrefix(strings.ToLower(value), "http://") || strings.HasPrefix(strings.ToLower(value), "https://") {
		if !strings.ContainsAny(value, " \n\r\t") {
			pushImageEntry(entries, imageEntry{Kind: "url", Value: value})
		}
	}
	if match := dataURLPattern.FindStringSubmatch(value); match != nil {
		pushImageEntry(entries, imageEntry{Kind: "base64", Value: match[2], MimeType: normalizeImageMimeType(match[1])})
	}
	for _, match := range markdownImagePattern.FindAllStringSubmatch(value, -1) {
		pushImageEntry(entries, imageEntry{Kind: "url", Value: match[1]})
	}
	for _, match := range imageURLPattern.FindAllString(value, -1) {
		if imageURLHasExtension(match) {
			pushImageEntry(entries, imageEntry{Kind: "url", Value: match})
		}
	}
}

func extractInlineImageEntry(value map[string]any) (imageEntry, bool) {
	for _, key := range []string{"inlineData", "inline_data", "imageData", "image_data"} {
		if nested, ok := value[key].(map[string]any); ok {
			if entry, ok := readInlineImageObject(nested); ok {
				return entry, true
			}
		}
	}
	return readInlineImageObject(value)
}

func readInlineImageObject(value map[string]any) (imageEntry, bool) {
	mimeType := firstString(value, "mimeType", "mime_type", "mediaType", "media_type", "contentType", "content_type")
	data := firstString(value, "data", "bytes")
	mimeType = normalizeImageMimeType(mimeType)
	if data == "" || !isSupportedImageMimeType(mimeType) {
		return imageEntry{}, false
	}
	if match := anchoredDataURLPattern.FindStringSubmatch(data); match != nil {
		return imageEntry{Kind: "base64", Value: match[2], MimeType: normalizeImageMimeType(match[1])}, true
	}
	return imageEntry{Kind: "base64", Value: data, MimeType: mimeType}, true
}

func pushImageEntry(entries *[]imageEntry, entry imageEntry) {
	if entry.Kind == "" || entry.Value == "" {
		return
	}
	for _, existing := range *entries {
		if existing.Kind == entry.Kind && existing.Value == entry.Value {
			return
		}
	}
	*entries = append(*entries, entry)
}

func firstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := value[key].(string); ok {
			return text
		}
	}
	return ""
}

func decodeBase64Image(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if match := anchoredDataURLPattern.FindStringSubmatch(value); match != nil {
		value = match[2]
	}
	return base64.StdEncoding.DecodeString(value)
}

func normalizeImageMimeType(value string) string {
	if value == "image/jpg" {
		return "image/jpeg"
	}
	return value
}

func isSupportedImageMimeType(value string) bool {
	return value == "image/png" || value == "image/jpeg" || value == "image/webp"
}

func imageURLHasExtension(value string) bool {
	lower := strings.ToLower(value)
	for _, extension := range []string{".png", ".jpg", ".jpeg", ".webp"} {
		index := strings.Index(lower, extension)
		if index >= 0 {
			after := lower[index+len(extension):]
			return after == "" || strings.HasPrefix(after, "?") || strings.HasPrefix(after, "#")
		}
	}
	return false
}

func dataURL(mimeType string, bytes []byte) string {
	if mimeType == "" {
		mimeType = "image/png"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(bytes)
}

func ensureTrailingSlash(value string) string {
	if strings.HasSuffix(value, "/") {
		return value
	}
	return value + "/"
}

func redact(bytes []byte, secrets ...string) string {
	message := string(bytes)
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	return message
}

func extractText(value any) string {
	parts := []string{}
	collectText(value, &parts)
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func collectText(value any, parts *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		if choices, ok := typed["choices"].([]any); ok {
			for _, choice := range choices {
				collectText(choice, parts)
			}
		}
		if candidates, ok := typed["candidates"].([]any); ok {
			for _, candidate := range candidates {
				collectText(candidate, parts)
			}
		}
		if message, ok := typed["message"].(map[string]any); ok {
			collectText(message["content"], parts)
		}
		if content, ok := typed["content"].(map[string]any); ok {
			collectText(content["parts"], parts)
		}
		appendText(typed["text"], parts)
		appendText(typed["output_text"], parts)
	case []any:
		for _, item := range typed {
			collectText(item, parts)
		}
	case string:
		appendText(typed, parts)
	}
}

func appendText(value any, parts *[]string) {
	text, ok := value.(string)
	if !ok {
		return
	}
	text = strings.TrimSpace(text)
	if text != "" {
		*parts = append(*parts, text)
	}
}


func isGrokImagineModel(modelID string) bool {
	normalized := normalizeGrokModelID(modelID)
	return normalized == "grok-imagine" ||
		normalized == "grok-imagine-edit" ||
		strings.HasPrefix(normalized, "grok-imagine-image")
}

func (p OpenAIProvider) requestBaseURL(snapshot channels.Snapshot) string {
	if strings.TrimSpace(p.InternalBaseURL) == "" {
		return snapshot.BaseURL
	}
	parsed, err := url.Parse(snapshot.BaseURL)
	if err != nil || !strings.EqualFold(parsed.Hostname(), "api.togoapi.com") {
		return snapshot.BaseURL
	}
	return strings.TrimRight(strings.TrimSpace(p.InternalBaseURL), "/")
}

func normalizeGrokModelID(modelID string) string {
	normalized := strings.ToLower(strings.TrimSpace(modelID))
	for _, prefix := range []string{"xai/", "x-ai/", "grok/"} {
		if strings.HasPrefix(normalized, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(normalized, prefix))
		}
	}
	return normalized
}

func applyImageGeometryFields(set func(name, value string), modelID string, request ImageRequest, isEdit bool) {
	if isGrokImagineModel(modelID) {
		set("response_format", "b64_json")
		if request.AspectRatio != "" {
			set("aspect_ratio", request.AspectRatio)
		}
		if request.Resolution != "" {
			set("resolution", request.Resolution)
		}
		if !isEdit && request.Quality != "" {
			set("quality", request.Quality)
		}
		return
	}
	if request.Size != "" {
		set("size", request.Size)
	}
	if request.Quality != "" {
		set("quality", request.Quality)
	}
}

func rewriteImageUpstreamError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "images api is not supported"):
		return errors.New("当前密钥所属分组不支持生图。Grok 生图需要使用已开启生图的 Grok 分组密钥。")
	case strings.Contains(message, "image generation is not enabled for this group"):
		return errors.New("当前密钥所属分组未开启生图，请联系管理员开启后再试。")
	default:
		return err
	}
}

func isGeminiChatImageModel(modelID string) bool {
	normalized := strings.ToLower(modelID)
	return strings.Contains(normalized, "gemini") && (strings.Contains(normalized, "image") || strings.Contains(normalized, "banana"))
}

func isGPTImageModel(modelID string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(modelID)), "gpt-image-")
}

func inputImageFilename(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return "input.jpg"
	case "image/webp":
		return "input.webp"
	default:
		return "input.png"
	}
}

func shouldFallbackToChatImage(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "only imagen models") || strings.Contains(message, "not supported model for image generation")
}

func isAihubMixDoubaoSeedream(snapshot channels.Snapshot) bool {
	parsed, err := url.Parse(snapshot.BaseURL)
	if err != nil {
		return false
	}
	hostname := strings.ToLower(parsed.Hostname())
	modelID := strings.ToLower(snapshot.ModelID)
	return strings.Contains(hostname, "aihubmix.com") && strings.Contains(modelID, "doubao-seedream")
}

func normalizeDoubaoSize(size string) string {
	normalized := strings.TrimSpace(strings.ToLower(size))
	switch normalized {
	case "4k":
		return "4K"
	case "2k":
		return "2K"
	case "auto":
		return "auto"
	default:
		return "2K"
	}
}

var (
	dataURLPattern         = regexp.MustCompile(`data:(image/(?:png|jpeg|jpg|webp));base64,([A-Za-z0-9+/=_-]+)`)
	anchoredDataURLPattern = regexp.MustCompile(`^data:(image/(?:png|jpeg|jpg|webp));base64,([A-Za-z0-9+/=_-]+)$`)
	markdownImagePattern   = regexp.MustCompile(`!\[[^\]]*]\((https?://[^)\s]+)\)`)
	imageURLPattern        = regexp.MustCompile(`https?://[^\s)"'<>]+`)
)
