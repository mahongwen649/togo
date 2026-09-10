package generation

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"imgtool/server/internal/channels"
)

func TestOpenAIProviderGenerateImageFromBase64(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("png"))},
			},
		})
	}))
	defer server.Close()

	provider := OpenAIProvider{Client: server.Client()}
	images, err := provider.GenerateImage(channels.Snapshot{
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		ModelID:    "gpt-image-1",
		Capability: "text-to-image",
	}, ImageRequest{
		Prompt:  "draw a whale",
		Size:    "1024x1024",
		Quality: "high",
		Count:   1,
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if len(images) != 1 || string(images[0].Bytes) != "png" || images[0].MimeType != "image/png" {
		t.Fatalf("images = %#v", images)
	}
	if requestBody["model"] != "gpt-image-1" || requestBody["prompt"] != "draw a whale" {
		t.Fatalf("unexpected request body: %#v", requestBody)
	}
	if requestBody["size"] != "1024x1024" {
		t.Fatalf("size = %#v, want 1024x1024", requestBody["size"])
	}
	if requestBody["quality"] != "high" {
		t.Fatalf("quality = %#v, want high", requestBody["quality"])
	}
}

func TestOpenAIProviderGenerateImageIncludesCountWhenRequestingOneImage(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("png"))},
			},
		})
	}))
	defer server.Close()

	provider := OpenAIProvider{Client: server.Client()}
	_, err := provider.GenerateImage(channels.Snapshot{
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		ModelID:    "gpt-image-2",
		Capability: "text-to-image",
	}, ImageRequest{
		Prompt: "draw a whale",
		Size:   "1024x1024",
		Count:  1,
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if requestBody["n"] != float64(1) {
		t.Fatalf("single image request should include n like legacy adapter: %#v", requestBody)
	}
}

func TestOpenAIProviderGenerateImageDoesNotForceGPTImageStreamingFields(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("png"))},
			},
		})
	}))
	defer server.Close()

	provider := OpenAIProvider{Client: server.Client()}
	_, err := provider.GenerateImage(channels.Snapshot{
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		ModelID:    "gpt-image-2",
		Capability: "text-to-image",
	}, ImageRequest{
		Prompt: "draw a whale",
		Size:   "1024x1024",
		Count:  1,
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if _, ok := requestBody["stream"]; ok {
		t.Fatalf("gpt-image request should not force stream field: %#v", requestBody)
	}
	if _, ok := requestBody["response_format"]; ok {
		t.Fatalf("gpt-image request should not force response_format field: %#v", requestBody)
	}
}

func TestOpenAIProviderGenerateImageParsesStreamedImageEvents(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		eventPayload, err := json.Marshal(map[string]any{
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("stream-png"))},
			},
		})
		if err != nil {
			t.Fatalf("marshal event: %v", err)
		}
		_, _ = fmt.Fprintf(w, "event: image_generation.completed\ndata: %s\n\n", eventPayload)
	}))
	defer server.Close()

	provider := OpenAIProvider{Client: server.Client()}
	images, err := provider.GenerateImage(channels.Snapshot{
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		ModelID:    "gpt-image-2",
		Capability: "text-to-image",
	}, ImageRequest{
		Prompt: "draw a whale",
		Size:   "1024x1024",
		Count:  1,
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if _, ok := requestBody["stream"]; ok {
		t.Fatalf("gpt-image request should not need stream field to parse streamed responses: %#v", requestBody)
	}
	if len(images) != 1 || string(images[0].Bytes) != "stream-png" {
		t.Fatalf("images = %#v", images)
	}
}

func TestOpenAIProviderGenerateImageLogsUpstreamTimingBreakdown(t *testing.T) {
	var logs bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previousOutput)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		time.Sleep(10 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("png"))},
			},
		})
	}))
	defer server.Close()

	provider := OpenAIProvider{Client: server.Client()}
	_, err := provider.GenerateImage(channels.Snapshot{
		ChannelName: "OpenAI",
		BaseURL:     server.URL,
		APIKey:      "sk-test",
		ModelID:     "gpt-image-2",
		Capability:  "text-to-image",
	}, ImageRequest{
		Prompt: "draw a whale",
		Size:   "1024x1024",
		Count:  1,
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	message := logs.String()
	for _, field := range []string{"response_header_ms=", "first_body_byte_ms=", "read_body_ms=", "response_bytes="} {
		if !strings.Contains(message, field) {
			t.Fatalf("log missing %s: %s", field, message)
		}
	}
}

func TestOpenAIProviderGenerateImageClampsMultipleImagesToOne(t *testing.T) {
	var requestBodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var requestBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requestBodies = append(requestBodies, requestBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("png-1"))},
			},
		})
	}))
	defer server.Close()

	provider := OpenAIProvider{Client: server.Client()}
	images, err := provider.GenerateImage(channels.Snapshot{
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		ModelID:    "gpt-image-2",
		Capability: "text-to-image",
	}, ImageRequest{
		Prompt: "draw a whale",
		Size:   "1024x1024",
		Count:  3,
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if len(requestBodies) != 1 {
		t.Fatalf("request count = %d, want 1: %#v", len(requestBodies), requestBodies)
	}
	if requestBodies[0]["n"] != float64(1) {
		t.Fatalf("image request should clamp n to one: %#v", requestBodies[0])
	}
	if len(images) != 1 || string(images[0].Bytes) != "png-1" {
		t.Fatalf("images = %#v", images)
	}
}

func TestOpenAIProviderGenerateImageDownloadsURL(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/webp")
		_, _ = w.Write([]byte("webp"))
	}))
	defer imageServer.Close()
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"url": imageServer.URL + "/result.webp"},
			},
		})
	}))
	defer apiServer.Close()

	provider := OpenAIProvider{Client: apiServer.Client()}
	images, err := provider.GenerateImage(channels.Snapshot{
		BaseURL:    apiServer.URL,
		APIKey:     "sk-test",
		ModelID:    "gpt-image-1",
		Capability: "text-to-image",
	}, ImageRequest{Prompt: "draw", Count: 1})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if len(images) != 1 || string(images[0].Bytes) != "webp" || images[0].MimeType != "image/webp" {
		t.Fatalf("images = %#v", images)
	}
}

func TestOpenAIProviderGenerateImageEditUsesMultipart(t *testing.T) {
	var sawMultipart bool
	var sawImage bool
	var sawSingleCount bool
	var sawGPTImageFields bool
	var sawImageContentType bool
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/edits" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Fatalf("content type = %s", r.Header.Get("Content-Type"))
		}
		if err := r.ParseMultipartForm(12 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		requestCount += 1
		sawMultipart = r.FormValue("model") == "gpt-image-2" && r.FormValue("prompt") == "edit this"
		sawGPTImageFields = r.FormValue("stream") != "" || r.FormValue("response_format") != ""
		sawSingleCount = r.FormValue("n") == "1"
		file, header, err := r.FormFile("image")
		if err != nil {
			t.Fatalf("missing image: %v", err)
		}
		defer file.Close()
		bytes, _ := io.ReadAll(file)
		sawImage = string(bytes) == "fake-image"
		sawImageContentType = header.Header.Get("Content-Type") == "image/webp"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("png-1"))},
			},
		})
	}))
	defer server.Close()

	provider := OpenAIProvider{Client: server.Client()}
	images, err := provider.GenerateImage(channels.Snapshot{
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		ModelID:    "gpt-image-2",
		Capability: "image-to-image",
	}, ImageRequest{
		Prompt:     "edit this",
		Size:       "1024x1024",
		Count:      2,
		Capability: "image-to-image",
		Image:      []byte("fake-image"),
		MimeType:   "image/webp",
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if requestCount != 1 || !sawMultipart || !sawImage || !sawSingleCount || sawGPTImageFields || !sawImageContentType || len(images) != 1 {
		t.Fatalf("multipart=%v image=%v gptImageFields=%v imageContentType=%v images=%#v", sawMultipart, sawImage, sawGPTImageFields, sawImageContentType, images)
	}
	if string(images[0].Bytes) != "png-1" {
		t.Fatalf("image = %#v", images[0])
	}
}

func TestOpenAIProviderGenerateImageUsesChatForGeminiInlineData(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{"text": "done"},
							{
								"inlineData": map[string]any{
									"mimeType": "image/png",
									"data":     base64.StdEncoding.EncodeToString([]byte("gemini-image")),
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	provider := OpenAIProvider{Client: server.Client()}
	images, err := provider.GenerateImage(channels.Snapshot{
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		ModelID:    "gemini-3-pro-image-preview",
		Capability: "text-to-image",
	}, ImageRequest{Prompt: "draw", Size: "1024x1024", Count: 1})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if requestBody["model"] != "gemini-3-pro-image-preview" {
		t.Fatalf("model = %#v", requestBody["model"])
	}
	if len(images) != 1 || string(images[0].Bytes) != "gemini-image" || images[0].MimeType != "image/png" {
		t.Fatalf("images = %#v", images)
	}
}

func TestOpenAIProviderGenerateImageFallsBackToChatMarkdownImage(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/webp")
		_, _ = w.Write([]byte("chat-webp"))
	}))
	defer imageServer.Close()

	var paths []string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/v1/images/generations" {
			http.Error(w, `{"error":{"message":"not supported model for image generation"}}`, http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "Here is it: ![result](" + imageServer.URL + "/result.webp)",
					},
				},
			},
		})
	}))
	defer apiServer.Close()

	provider := OpenAIProvider{Client: apiServer.Client()}
	images, err := provider.GenerateImage(channels.Snapshot{
		BaseURL:    apiServer.URL,
		APIKey:     "sk-test",
		ModelID:    "gpt-image-compatible-chat",
		Capability: "text-to-image",
	}, ImageRequest{Prompt: "draw", Count: 1})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if strings.Join(paths, ",") != "/v1/images/generations,/v1/chat/completions" {
		t.Fatalf("paths = %#v", paths)
	}
	if len(images) != 1 || string(images[0].Bytes) != "chat-webp" || images[0].MimeType != "image/webp" {
		t.Fatalf("images = %#v", images)
	}
}

func TestOpenAIProviderGenerateImageUsesAihubMixDoubaoPredictions(t *testing.T) {
	var apiRequestPath string
	var authHeader string
	var requestBody map[string]any
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "api.aihubmix.com":
			apiRequestPath = r.URL.Path
			authHeader = r.Header.Get("Authorization")
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return jsonResponse(200, map[string]any{
				"output": []map[string]any{
					{"url": "https://cdn.example.com/result.png"},
				},
			}), nil
		case "cdn.example.com":
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"image/png"}},
				Body:       io.NopCloser(bytes.NewReader([]byte("doubao-png"))),
			}, nil
		default:
			t.Fatalf("unexpected host: %s", r.URL.Host)
			return nil, nil
		}
	})}

	provider := OpenAIProvider{Client: client}
	images, err := provider.GenerateImage(channels.Snapshot{
		BaseURL:    "https://api.aihubmix.com",
		APIKey:     "sk-test",
		ModelID:    "doubao-seedream-3-0-t2i",
		Capability: "text-to-image",
	}, ImageRequest{
		Prompt:   "draw",
		Size:     "4k",
		Count:    1,
		Image:    []byte("reference"),
		MimeType: "image/webp",
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if apiRequestPath != "/v1/models/doubao/doubao-seedream-3-0-t2i/predictions" {
		t.Fatalf("path = %s", apiRequestPath)
	}
	if authHeader != "Bearer sk-test" {
		t.Fatalf("Authorization = %q", authHeader)
	}
	input, ok := requestBody["input"].(map[string]any)
	if !ok {
		t.Fatalf("missing input object: %#v", requestBody)
	}
	if input["prompt"] != "draw" || input["size"] != "4K" || input["response_format"] != "url" || input["watermark"] != false {
		t.Fatalf("unexpected input: %#v", input)
	}
	if input["image"] != "data:image/webp;base64,cmVmZXJlbmNl" {
		t.Fatalf("image = %#v", input["image"])
	}
	if len(images) != 1 || string(images[0].Bytes) != "doubao-png" || images[0].MimeType != "image/png" {
		t.Fatalf("images = %#v", images)
	}
}

func TestExtractImageEntriesMatchesDesktopAdapterShapes(t *testing.T) {
	entries := extractImageEntries(map[string]any{
		"output": []any{
			map[string]any{"url": "https://example.com/download?token=abc"},
		},
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"content": "![result](https://cdn.example.com/result.webp) data:image/png;base64,aGVsbG8=",
				},
			},
		},
		"candidates": []any{
			map[string]any{
				"content": map[string]any{
					"parts": []any{
						map[string]any{
							"inlineData": map[string]any{
								"mimeType": "image/jpg",
								"data":     "aW1hZ2U=",
							},
						},
					},
				},
			},
		},
	})
	want := []imageEntry{
		{Kind: "url", Value: "https://example.com/download?token=abc"},
		{Kind: "url", Value: "https://cdn.example.com/result.webp"},
		{Kind: "base64", Value: "aGVsbG8=", MimeType: "image/png"},
		{Kind: "base64", Value: "aW1hZ2U=", MimeType: "image/jpeg"},
	}
	if len(entries) != len(want) {
		t.Fatalf("entries length = %d, want %d: %#v", len(entries), len(want), entries)
	}
	for _, expected := range want {
		found := false
		for _, entry := range entries {
			if entry == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing entry %#v in %#v", expected, entries)
		}
	}
}

func TestAihubMixDoubaoDetectionAndSizeNormalization(t *testing.T) {
	if !isAihubMixDoubaoSeedream(channels.Snapshot{BaseURL: "https://api.aihubmix.com", ModelID: "doubao-seedream-3-0-i2i"}) {
		t.Fatal("expected aihubmix doubao seedream model to be detected")
	}
	if isAihubMixDoubaoSeedream(channels.Snapshot{BaseURL: "https://api.openai.com", ModelID: "doubao-seedream-3-0-i2i"}) {
		t.Fatal("non-aihubmix host should not use doubao predictions endpoint")
	}
	cases := map[string]string{
		"4k":     "4K",
		" 2K ":   "2K",
		"auto":   "auto",
		"1024x1": "2K",
		"":       "2K",
	}
	for input, want := range cases {
		if got := normalizeDoubaoSize(input); got != want {
			t.Fatalf("normalizeDoubaoSize(%q) = %q, want %q", input, got, want)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, body any) *http.Response {
	var buffer bytes.Buffer
	_ = json.NewEncoder(&buffer).Encode(body)
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(&buffer),
	}
}

func TestOpenAIProviderGenerateGrokImageSendsNativeGeometry(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("png"))},
			},
		})
	}))
	defer server.Close()

	provider := OpenAIProvider{Client: server.Client()}
	_, err := provider.GenerateImage(channels.Snapshot{
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		ModelID:    "grok-imagine-image-2.0",
		Capability: "text-to-image",
	}, ImageRequest{
		Prompt:      "a neon city",
		AspectRatio: "16:9",
		Resolution:  "2k",
		Quality:     "medium",
		Size:        "1024x1024",
		Count:       1,
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if requestBody["aspect_ratio"] != "16:9" || requestBody["resolution"] != "2k" || requestBody["quality"] != "medium" {
		t.Fatalf("unexpected grok geometry: %#v", requestBody)
	}
	if requestBody["response_format"] != "b64_json" {
		t.Fatalf("grok text-to-image should request b64_json: %#v", requestBody)
	}
	if _, ok := requestBody["size"]; ok {
		t.Fatalf("grok text-to-image should not send size: %#v", requestBody)
	}
}

func TestOpenAIProviderGenerateGrokImageEditOmitsQualityAndSize(t *testing.T) {
	var formValues url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/edits" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(12 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		formValues = r.Form
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("png"))},
			},
		})
	}))
	defer server.Close()

	provider := OpenAIProvider{Client: server.Client()}
	_, err := provider.GenerateImage(channels.Snapshot{
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		ModelID:    "grok-imagine-image-2.0",
		Capability: "image-to-image",
	}, ImageRequest{
		Prompt:      "edit this",
		AspectRatio: "auto",
		Resolution:  "1k",
		Quality:     "medium",
		Size:        "1024x1024",
		Capability:  "image-to-image",
		Image:       []byte("fake-image"),
		MimeType:    "image/png",
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if formValues.Get("aspect_ratio") != "auto" || formValues.Get("resolution") != "1k" {
		t.Fatalf("unexpected grok edit geometry: %#v", formValues)
	}
	if formValues.Get("response_format") != "b64_json" {
		t.Fatalf("grok edit should request b64_json: %#v", formValues)
	}
	if formValues.Get("quality") != "" || formValues.Get("size") != "" {
		t.Fatalf("grok edit should omit quality and size: %#v", formValues)
	}
}

func TestIsGrokImagineModelAcceptsProviderPrefixes(t *testing.T) {
	for _, modelID := range []string{
		"grok-imagine-image-2.0",
		"xai/grok-imagine-image-2.0",
		"x-ai/grok-imagine-image-quality",
		"grok/grok-imagine-image",
	} {
		if !isGrokImagineModel(modelID) {
			t.Errorf("isGrokImagineModel(%q) = false", modelID)
		}
	}
}

func TestOpenAIProviderRequestBaseURLOnlyRewritesPublicCoreHost(t *testing.T) {
	provider := OpenAIProvider{InternalBaseURL: "http://127.0.0.1:8080/"}
	if got := provider.requestBaseURL(channels.Snapshot{BaseURL: "https://api.togoapi.com"}); got != "http://127.0.0.1:8080" {
		t.Fatalf("requestBaseURL public Core = %q", got)
	}
	if got := provider.requestBaseURL(channels.Snapshot{BaseURL: "https://api.openai.com"}); got != "https://api.openai.com" {
		t.Fatalf("requestBaseURL external provider = %q", got)
	}
}

func TestRewriteImageUpstreamError(t *testing.T) {
	err := rewriteImageUpstreamError(fmt.Errorf("upstream error 404: Images API is not supported for this platform"))
	if err == nil || !strings.Contains(err.Error(), "Grok 分组") {
		t.Fatalf("unsupported platform error = %v", err)
	}
	err = rewriteImageUpstreamError(fmt.Errorf("upstream error 403: Image generation is not enabled for this group"))
	if err == nil || !strings.Contains(err.Error(), "未开启生图") {
		t.Fatalf("permission error = %v", err)
	}
}
