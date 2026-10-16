package httpapi

import (
	"context"
	"strings"
	"testing"
)

func TestExclusivePageWrappedAnswers(t *testing.T) {
	document := `<!DOCTYPE html><html lang="zh"><body><svg></svg><script>const s="</html>";</script></body></html>`
	for _, answer := range []string{
		"下面是生成的页面：\n```html\n" + document + "\n```\n保存后即可打开。",
		"下面是代码：\n" + document + "\n结束。",
		"说明\n~~~HTML\n" + document + "\n~~~\n说明",
		"```js\nconst value = '<html>wrong</html>';\n```\n```html\n" + document + "\n```",
	} {
		got, err := parseExclusivePage([]byte(pageResponse(answer)))
		if err != nil || got != document {
			t.Fatalf("failed extraction: %s, %v", got, err)
		}
	}
}
func TestExclusivePageFragmentsAndMessageParts(t *testing.T) {
	for _, answer := range []string{"<html><body><svg></svg></body>", "```svg\n<svg viewBox=\"0 0 1 1\"><circle r=\"1\"/></svg>\n```", "<body><svg></svg></body>"} {
		got, err := parseExclusivePage([]byte(pageResponse(answer)))
		if err != nil || !strings.Contains(got, "<svg") || !strings.Contains(got, "</html>") {
			t.Fatal(got, err)
		}
	}
	body := `{"status":"completed","output":[{"type":"message","phase":"commentary","content":[{"type":"output_text","text":"我会生成页面。"}]},{"type":"message","phase":"final_answer","role":"assistant","content":[{"type":"output_text","text":"这是页面：\n<html><body>"},{"type":"output_text","text":"final</body></html>\n结束"}]}]}`
	got, err := parseExclusivePage([]byte(body))
	if err != nil || got != "<html><body>final</body></html>" {
		t.Fatal(got, err)
	}
	for _, answer := range []string{"<html><body><script>unfinished", "<svg><g>", "<html>\x00</html>", "普通文字，没有页面"} {
		if _, err := parseExclusivePage([]byte(pageResponse(answer))); err == nil {
			t.Fatal("accepted incomplete answer", answer)
		}
	}
}

func TestExclusiveAccountTestReportsPlainTextAndUpstreamErrors(t *testing.T) {
	_, message, err := parseExclusiveAccountTest([]byte("data: {\"type\":\"content\",\"text\":\"hello\"}\n\ndata: {\"type\":\"test_complete\",\"success\":true}\n\n"))
	if err == nil || !strings.Contains(message, "只返回了普通文本") {
		t.Fatalf("plain text result was not explained: message=%q err=%v", message, err)
	}

	_, message, err = parseExclusiveAccountTest([]byte("data: {\"type\":\"error\",\"error\":\"model not found\"}\n\n"))
	if err == nil || message != "model not found" {
		t.Fatalf("upstream error was not preserved: message=%q err=%v", message, err)
	}
}

func TestExclusivePageDiagnosticIsPrivate(t *testing.T) {
	s, fake, db, channel, _ := pageFixture(t)
	fake.response = `{"status":"completed","output":[{"type":"reasoning","content":[{"type":"output_text","text":"private reasoning"}]},{"type":"message","content":[{"type":"output_text","text":"reply without markup"}]}]}`
	if w := pageStart(s, channel, "holder"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	close(fake.finish)
	waitPage(t, s, channel, "failed")
	var secret string
	if err := db.QueryRow(`SELECT response_secret FROM portal_exclusive_page_tests WHERE channel_id=$1`, channel).Scan(&secret); err != nil {
		t.Fatal(err)
	}
	if secret == "" || strings.Contains(secret, "reply without markup") {
		t.Fatal("diagnostic was not encrypted")
	}
	value, err := s.openExclusive(secret)
	if err != nil || !strings.Contains(value, "reply without markup") || strings.Contains(value, "private reasoning") {
		t.Fatal("invalid diagnostic", err)
	}
	v, err := s.exclusivePageMetadata(context.Background(), channel, true)
	if err != nil || strings.Contains(exclusiveJSON(v), "reply without markup") || strings.Contains(exclusiveJSON(v), secret) {
		t.Fatal("public diagnostic leak", err)
	}
}
