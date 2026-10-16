package httpapi

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestNormalizeAccountProductValidity(t *testing.T) {
	for _, days := range []int{1, 7, 30, 3650} {
		got, err := normalizeAccountProductValidity("rental", days)
		if err != nil || got != days {
			t.Fatalf("rental days=%d got=%d err=%v", days, got, err)
		}
	}
	for _, days := range []int{-1, 0, 3651} {
		if _, err := normalizeAccountProductValidity("rental", days); err == nil {
			t.Fatalf("accepted invalid rental days %d", days)
		}
	}
	if got, err := normalizeAccountProductValidity("purchase", 30); err != nil || got != 0 {
		t.Fatalf("purchase validity got=%d err=%v", got, err)
	}
	if _, err := normalizeAccountProductValidity("unknown", 30); err == nil {
		t.Fatal("accepted unknown product mode")
	}
}

func TestNormalizeAccountProductImage(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), []byte("product-banner")...)
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	got, err := normalizeAccountProductImage(dataURL)
	if err != nil || got != dataURL {
		t.Fatalf("valid upload rejected: got=%q err=%v", got, err)
	}
	if _, err := normalizeAccountProductImage("data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte("<svg/>"))); err == nil {
		t.Fatal("accepted SVG upload")
	}
	oversized := make([]byte, maxAccountProductImageBytes+1)
	copy(oversized, png)
	if _, err := normalizeAccountProductImage("data:image/png;base64," + base64.StdEncoding.EncodeToString(oversized)); err == nil {
		t.Fatal("accepted oversized upload")
	}
	if got, err := normalizeAccountProductImage("https://cdn.example.com/banner.png"); err != nil || !strings.HasPrefix(got, "https://") {
		t.Fatalf("legacy URL rejected: got=%q err=%v", got, err)
	}
	if _, err := normalizeAccountProductImage("javascript:alert(1)"); err == nil {
		t.Fatal("accepted unsafe image URL")
	}
}
