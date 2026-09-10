package storage

import (
	"path"
	"regexp"
	"strings"
	"time"

	"imgtool/server/internal/generation"
)

type LocalMetadataStorage struct {
	bucket string
	prefix string
	now    func() time.Time
}

func NewLocalMetadataStorage(bucket string, prefix string, now func() time.Time) *LocalMetadataStorage {
	if now == nil {
		now = time.Now
	}
	return &LocalMetadataStorage{bucket: bucket, prefix: prefix, now: now}
}

func (s *LocalMetadataStorage) SaveGeneratedImage(input generation.StoreImageInput) (generation.StoredImage, error) {
	return generation.StoredImage{
		StorageProvider: "local-metadata",
		Bucket:          s.bucket,
		ObjectKey:       ObjectKey(s.prefix, s.now(), input),
		Role:            normalizedRole(input.Role),
		MediaType:       "image",
		MimeType:        normalizeMime(input.MimeType),
		SizeBytes:       int64(len(input.Bytes)),
	}, nil
}

func (s *LocalMetadataStorage) DeleteObject(_ string) error {
	return nil
}

func ObjectKey(prefix string, now time.Time, input generation.StoreImageInput) string {
	prefix = strings.Trim(path.Clean(strings.ReplaceAll(prefix, "\\", "/")), "/")
	if prefix == "." {
		prefix = "aiImg"
	}
	date := now.Format("2006-01-02")
	clock := now.Format("150405")
	role := normalizedRole(input.Role)
	nameParts := []string{clock, slug(input.TaskID)}
	if role == "reference" {
		nameParts = append(nameParts, "reference")
	}
	nameParts = append(nameParts, intString(input.Index))
	fileName := strings.Join(nameParts, "-") + "." + extensionFor(input.MimeType)
	return path.Join(prefix, "users", slug(input.UserID), slug(input.ChannelSlug), slug(input.ModelSlug), date, fileName)
}

func normalizeMime(value string) string {
	value = strings.Split(value, ";")[0]
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "application/octet-stream"
	}
	return value
}

func extensionFor(mimeType string) string {
	switch normalizeMime(mimeType) {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/webp":
		return "webp"
	default:
		return "bin"
	}
}

var slugPattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func slug(value string) string {
	value = slugPattern.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "item"
	}
	if len(value) > 80 {
		return value[:80]
	}
	return value
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	negative := value < 0
	if negative {
		value = -value
	}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
