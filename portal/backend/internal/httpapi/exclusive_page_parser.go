package httpapi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// parseExclusiveAccountTest extracts the streamed Core account-test result.
// Core emits one JSON event per SSE line; only content events are retained so
// status messages cannot be mistaken for generated markup.
func parseExclusiveAccountTest(body []byte) (string, string, error) {
	if len(body) > exclusivePageLimit {
		return "", "", errors.New("生成响应过大，未替换已有页面")
	}
	var text strings.Builder
	var failure string
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 1024), exclusivePageLimit+1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var event struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Error   string `json:"error"`
			Success *bool  `json:"success"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &event); err != nil {
			continue
		}
		if event.Type == "content" {
			text.WriteString(event.Text)
		}
		if event.Type == "error" || (event.Type == "test_complete" && event.Success != nil && !*event.Success) {
			failure = event.Error
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}
	if failure != "" {
		return "", failure, errors.New(failure)
	}
	for _, candidate := range exclusivePageCandidates(text.String()) {
		if page, ok := extractExclusiveDocument(candidate); ok {
			return page, "", nil
		}
	}
	if strings.TrimSpace(text.String()) != "" {
		return "", "账号测试已连通上游，但模型只返回了普通文本，未按要求生成 HTML 或 SVG 页面", errors.New("account test returned plain text instead of HTML or SVG")
	}
	return "", "", errors.New("账号测试回复中未找到可展示的 HTML 或 SVG 页面")
}

type exclusivePageResponse struct {
	Status string `json:"status"`
	Output []struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Phase   string `json:"phase"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func parseExclusivePage(body []byte) (string, error) {
	if len(body) > exclusivePageLimit {
		return "", errors.New("生成响应过大，未替换已有页面")
	}
	var response exclusivePageResponse
	if json.Unmarshal(body, &response) != nil {
		return "", errors.New("Core 返回的不是有效 Responses JSON，未替换已有页面")
	}
	if response.Status != "completed" {
		return "", errors.New("模型生成未完成，未替换已有页面")
	}
	// Final answers take precedence over commentary. Do not join independent
	// messages: explanatory text and earlier drafts must not corrupt a valid page.
	for _, phase := range []string{"final_answer", ""} {
		for i := len(response.Output) - 1; i >= 0; i-- {
			o := response.Output[i]
			if o.Type != "message" || (o.Role != "" && o.Role != "assistant") || (phase != "" && o.Phase != phase) {
				continue
			}
			var text strings.Builder
			for _, c := range o.Content {
				if c.Type == "output_text" {
					text.WriteString(c.Text)
				}
			}
			for _, candidate := range exclusivePageCandidates(text.String()) {
				if page, ok := extractExclusiveDocument(candidate); ok {
					return page, nil
				}
			}
		}
	}
	return "", errors.New("回复中未找到可展示的 HTML 或 SVG 页面，诊断信息已保留")
}

// Treat code fences as containers, not as a required response prefix. Ignore
// non-HTML fences so HTML-looking strings inside JavaScript are not mistaken for a page.
func exclusivePageCandidates(text string) []string {
	var candidates []string
	var outside, block strings.Builder
	marker, language := "", ""
	for _, line := range strings.SplitAfter(text, "\n") {
		trim := strings.TrimSpace(line)
		if marker == "" {
			if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
				char := trim[0]
				n := 0
				for n < len(trim) && trim[n] == char {
					n++
				}
				marker = trim[:n]
				language = strings.ToLower(strings.TrimSpace(trim[n:]))
				block.Reset()
			} else {
				outside.WriteString(line)
			}
		} else if len(trim) >= len(marker) && strings.Trim(trim, string(marker[0])) == "" {
			if language == "html" || language == "svg" || language == "" {
				candidates = append(candidates, block.String())
			}
			marker = ""
		} else {
			block.WriteString(line)
		}
	}
	candidates = append(candidates, outside.String())
	return candidates
}

func extractExclusiveDocument(text string) (string, bool) {
	if strings.ContainsRune(text, 0) {
		return "", false
	}
	z := html.NewTokenizer(strings.NewReader(text))
	offset, start, doctype, bodyStart, bodyEnd, svgStart, svgEnd, svgDepth := 0, -1, -1, -1, -1, -1, -1, 0
	for {
		kind := z.Next()
		raw := z.Raw()
		at := offset
		offset += len(raw)
		if kind == html.ErrorToken {
			if z.Err() != io.EOF {
				return "", false
			}
			break
		}
		token := z.Token()
		switch kind {
		case html.DoctypeToken:
			if strings.EqualFold(token.Data, "html") && start < 0 {
				doctype = at
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			switch token.Data {
			case "html":
				if start < 0 {
					start = at
					if doctype >= 0 {
						start = doctype
					}
				}
			case "body":
				if bodyStart < 0 {
					bodyStart = at
				}
			case "svg":
				if svgStart < 0 {
					svgStart = at
				}
				if kind == html.StartTagToken {
					svgDepth++
				} else if svgDepth == 0 {
					svgEnd = offset
				}
			}
		case html.EndTagToken:
			switch token.Data {
			case "html":
				if start >= 0 {
					return strings.TrimSpace(text[start:offset]), true
				}
			case "body":
				if bodyStart >= 0 {
					bodyEnd = offset
				}
			case "svg":
				if svgDepth > 0 {
					svgDepth--
					if svgDepth == 0 {
						svgEnd = offset
					}
				}
			}
		}
	}
	// HTML permits omitted closing html tags. Accept a closed body without inventing
	// missing script or body content. A standalone SVG is displayed inside a document.
	if bodyEnd > bodyStart && bodyStart >= 0 {
		if start >= 0 {
			return strings.TrimSpace(text[start:bodyEnd]) + "</html>", true
		}
		return "<!doctype html><html>" + text[bodyStart:bodyEnd] + "</html>", true
	}
	if start < 0 && svgStart >= 0 && svgEnd > svgStart {
		return "<!doctype html><html><head><meta charset=\"utf-8\"></head><body>" + text[svgStart:svgEnd] + "</body></html>", true
	}
	return "", false
}

func exclusivePageDiagnostic(body []byte) string {
	var response exclusivePageResponse
	if json.Unmarshal(body, &response) != nil {
		return exclusiveJSON(map[string]any{"response_bytes": len(body), "format": "invalid_json"})
	}
	// Persist only assistant output and structural metadata, never reasoning or
	// upstream headers. The database stores this diagnostic encrypted, not in public APIs.
	messages := []string{}
	types := []string{}
	for _, o := range response.Output {
		types = append(types, o.Type)
		if o.Type != "message" || (o.Role != "" && o.Role != "assistant") {
			continue
		}
		var text bytes.Buffer
		for _, c := range o.Content {
			if c.Type == "output_text" {
				text.WriteString(c.Text)
			}
		}
		messages = append(messages, text.String())
	}
	return exclusiveJSON(map[string]any{"status": response.Status, "response_bytes": len(body), "output_types": types, "messages": messages})
}
