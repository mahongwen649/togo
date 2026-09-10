package providers

import (
	"strings"
	"testing"
)

func TestFilterImageModelsPicksLatestGPTAndGrok(t *testing.T) {
	openai := filterImageModels("openai", []string{
		"gpt-4o",
		"gpt-image-1",
		"gpt-image-1.5",
		"gpt-image-2",
		"gpt-image-2.5-flare",
		"gpt-image-2.5-sunburst",
		"grok-imagine-image-2.0",
	})
	if len(openai) == 0 || openai[0] != "gpt-image-2.5-sunburst" {
		t.Fatalf("openai models = %#v", openai)
	}
	grok := filterImageModels("grok", []string{
		"grok-4.6",
		"grok-imagine-video",
		"grok-imagine",
		"grok-imagine-image",
		"grok-imagine-image-quality",
		"grok-imagine-image-2.0",
	})
	if len(grok) == 0 || grok[0] != "grok-imagine-image-2.0" {
		t.Fatalf("grok models = %#v", grok)
	}
	for _, id := range grok {
		if strings.Contains(id, "video") {
			t.Fatalf("video model leaked: %#v", grok)
		}
	}
}
