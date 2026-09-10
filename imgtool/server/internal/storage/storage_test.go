package storage

import (
	"strings"
	"testing"
	"time"

	"imgtool/server/internal/generation"
)

func TestObjectKeyUsesRequiredConvention(t *testing.T) {
	now := time.Date(2026, 7, 7, 15, 30, 22, 0, time.UTC)
	key := ObjectKey("aiImg", now, generation.StoreImageInput{
		UserID:      "usr_123",
		ChannelSlug: "OpenAI Provider",
		ModelSlug:   "gpt-image-1",
		TaskID:      "tsk_abc/123",
		Index:       2,
		MimeType:    "image/png",
	})

	expected := "aiImg/users/usr_123/OpenAI_Provider/gpt-image-1/2026-07-07/153022-tsk_abc_123-2.png"
	if key != expected {
		t.Fatalf("key = %q, want %q", key, expected)
	}
}

func TestLocalStorageReturnsObjectMetadata(t *testing.T) {
	store := NewLocalMetadataStorage("bucket", "aiImg", func() time.Time {
		return time.Date(2026, 7, 7, 15, 30, 22, 0, time.UTC)
	})

	stored, err := store.SaveGeneratedImage(generation.StoreImageInput{
		UserID:      "usr_123",
		ChannelSlug: "OpenAI",
		ModelSlug:   "gpt-image-1",
		TaskID:      "tsk_abc",
		Index:       0,
		Bytes:       []byte("png"),
		MimeType:    "image/png",
	})
	if err != nil {
		t.Fatalf("SaveGeneratedImage returned error: %v", err)
	}
	if stored.StorageProvider != "local-metadata" || stored.Bucket != "bucket" {
		t.Fatalf("unexpected storage metadata: %#v", stored)
	}
	if !strings.HasPrefix(stored.ObjectKey, "aiImg/users/usr_123/OpenAI/gpt-image-1/2026-07-07/153022-tsk_abc-0.png") {
		t.Fatalf("object key = %q", stored.ObjectKey)
	}
	if stored.SizeBytes != 3 || stored.MimeType != "image/png" || stored.MediaType != "image" {
		t.Fatalf("unexpected file metadata: %#v", stored)
	}
}

func TestLocalStorageDeleteObjectIsNoop(t *testing.T) {
	store := NewLocalMetadataStorage("bucket", "aiImg", nil)

	if err := store.DeleteObject("aiImg/users/alice/result.png"); err != nil {
		t.Fatalf("DeleteObject returned error: %v", err)
	}
}

func TestCOSBucketURLUsesConfiguredEndpoint(t *testing.T) {
	u, err := cosBucketURL(COSOptions{
		Bucket:   "togoapi-img-1385779910",
		Region:   "ap-guangzhou",
		Endpoint: "togoapi-img-1385779910.cos.ap-guangzhou.myqcloud.com",
	})
	if err != nil {
		t.Fatalf("cosBucketURL returned error: %v", err)
	}
	if got := u.String(); got != "https://togoapi-img-1385779910.cos.ap-guangzhou.myqcloud.com" {
		t.Fatalf("url = %q", got)
	}
}

func TestCOSBucketURLCanBeBuiltFromBucketAndRegion(t *testing.T) {
	u, err := cosBucketURL(COSOptions{
		Bucket: "togoapi-img-1385779910",
		Region: "ap-guangzhou",
	})
	if err != nil {
		t.Fatalf("cosBucketURL returned error: %v", err)
	}
	if got := u.String(); got != "https://togoapi-img-1385779910.cos.ap-guangzhou.myqcloud.com" {
		t.Fatalf("url = %q", got)
	}
}
