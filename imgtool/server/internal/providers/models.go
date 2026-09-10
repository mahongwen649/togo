package providers

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var gptImageVersion = regexp.MustCompile(`(?i)^gpt-image-(\d+)(?:\.(\d+))?`)
var grokImageVersion = regexp.MustCompile(`(?i)^grok-imagine-image-(\d+)(?:\.(\d+))?`)

func filterImageModels(providerID string, ids []string) []string {
	filtered := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		normalized := normalizeModelID(id)
		if seen[normalized] || !isProviderImageModel(providerID, normalized) {
			continue
		}
		seen[normalized] = true
		filtered = append(filtered, id)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		left, right := modelRank(filtered[i]), modelRank(filtered[j])
		if left != right {
			return left > right
		}
		return filtered[i] > filtered[j]
	})
	return filtered
}

func isProviderImageModel(providerID, normalized string) bool {
	switch providerID {
	case "openai":
		return strings.HasPrefix(normalized, "gpt-image-")
	case "grok":
		return isGrokImageModel(normalized)
	default:
		return false
	}
}

func isGrokImageModel(normalized string) bool {
	if strings.Contains(normalized, "video") {
		return false
	}
	return normalized == "grok-imagine" ||
		normalized == "grok-imagine-edit" ||
		strings.HasPrefix(normalized, "grok-imagine-image")
}

func normalizeModelID(modelID string) string {
	normalized := strings.ToLower(strings.TrimSpace(modelID))
	for _, prefix := range []string{"xai/", "x-ai/", "grok/"} {
		if strings.HasPrefix(normalized, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(normalized, prefix))
		}
	}
	return normalized
}

func modelRank(modelID string) int {
	normalized := normalizeModelID(modelID)
	if strings.HasPrefix(normalized, "gpt-image-") {
		major, minor := 1, 0
		if match := gptImageVersion.FindStringSubmatch(normalized); match != nil {
			major, _ = strconv.Atoi(match[1])
			if match[2] != "" {
				minor, _ = strconv.Atoi(match[2])
			}
		}
		extra := 0
		if strings.Contains(normalized, "sunburst") {
			extra += 20
		}
		if strings.Contains(normalized, "flare") {
			extra += 10
		}
		return major*10000 + minor*100 + extra
	}
	if match := grokImageVersion.FindStringSubmatch(normalized); match != nil {
		major, _ := strconv.Atoi(match[1])
		minor := 0
		if match[2] != "" {
			minor, _ = strconv.Atoi(match[2])
		}
		return major*10000 + minor*100
	}
	switch {
	case strings.Contains(normalized, "grok-imagine-image-quality"):
		return 1500
	case normalized == "grok-imagine-image" || strings.HasPrefix(normalized, "grok-imagine-image"):
		return 1000
	case normalized == "grok-imagine" || normalized == "grok-imagine-edit":
		return 500
	default:
		return 0
	}
}
