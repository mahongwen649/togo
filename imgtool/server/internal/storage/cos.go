package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	tencos "github.com/tencentyun/cos-go-sdk-v5"

	"imgtool/server/internal/generation"
)

type COSOptions struct {
	Bucket           string
	Region           string
	Endpoint         string
	Prefix           string
	SecretID         string
	SecretKey        string
	SignedURLExpires time.Duration
}

type COSStorage struct {
	client *tencos.Client
	opts   COSOptions
	now    func() time.Time
}

const (
	cosHTTPTimeout = 2 * time.Minute
	cosPartSizeMB  = 1
	cosWorkers     = 4
)

func NewCOSStorage(opts COSOptions, now func() time.Time) (*COSStorage, error) {
	if opts.Bucket == "" || opts.SecretID == "" || opts.SecretKey == "" {
		return nil, errors.New("cos storage is not configured")
	}
	if opts.Region == "" && opts.Endpoint == "" {
		return nil, errors.New("cos region or endpoint is required")
	}
	if opts.Prefix == "" {
		opts.Prefix = "aiImg"
	}
	if opts.SignedURLExpires <= 0 {
		opts.SignedURLExpires = 10 * time.Minute
	}
	if now == nil {
		now = time.Now
	}
	bucketURL, err := cosBucketURL(opts)
	if err != nil {
		return nil, err
	}
	client := tencos.NewClient(&tencos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Transport: &tencos.AuthorizationTransport{
			SecretID:  opts.SecretID,
			SecretKey: opts.SecretKey,
		},
		Timeout: cosHTTPTimeout,
	})
	return &COSStorage{client: client, opts: opts, now: now}, nil
}

func (s *COSStorage) SaveGeneratedImage(input generation.StoreImageInput) (generation.StoredImage, error) {
	mimeType := normalizeMime(input.MimeType)
	key := ObjectKey(s.opts.Prefix, s.now(), input)
	tempFile, err := os.CreateTemp("", "imgtool-cos-*")
	if err != nil {
		return generation.StoredImage{}, fmt.Errorf("create temporary image: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if _, err := tempFile.Write(input.Bytes); err != nil {
		_ = tempFile.Close()
		return generation.StoredImage{}, fmt.Errorf("write temporary image: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return generation.StoredImage{}, fmt.Errorf("close temporary image: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cosHTTPTimeout)
	defer cancel()
	_, _, err = s.client.Object.Upload(ctx, key, tempPath, &tencos.MultiUploadOptions{
		OptIni: &tencos.InitiateMultipartUploadOptions{
			ObjectPutHeaderOptions: &tencos.ObjectPutHeaderOptions{ContentType: mimeType},
		},
		PartSize:        cosPartSizeMB,
		ThreadPoolSize:  cosWorkers,
		DisableChecksum: true,
	})
	if err != nil {
		return generation.StoredImage{}, err
	}
	return generation.StoredImage{
		StorageProvider: "tencent-cos",
		Bucket:          s.opts.Bucket,
		ObjectKey:       key,
		Role:            normalizedRole(input.Role),
		MediaType:       "image",
		MimeType:        mimeType,
		SizeBytes:       int64(len(input.Bytes)),
	}, nil
}

func (s *COSStorage) SignedGetURL(objectKey string) (string, error) {
	result, err := s.client.Object.GetPresignedURL(context.Background(), http.MethodGet, objectKey, s.opts.SecretID, s.opts.SecretKey, s.opts.SignedURLExpires, nil)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

func (s *COSStorage) SignedDownloadURL(objectKey string, filename string) (string, error) {
	filename = strings.TrimSpace(strings.ReplaceAll(filename, `"`, ""))
	if filename == "" {
		filename = "image.png"
	}
	q := url.Values{}
	q.Set("response-content-disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	result, err := s.client.Object.GetPresignedURL(context.Background(), http.MethodGet, objectKey, s.opts.SecretID, s.opts.SecretKey, s.opts.SignedURLExpires, &tencos.PresignedURLOptions{
		Query: &q,
	})
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

func (s *COSStorage) ReadObject(objectKey string) ([]byte, error) {
	result, err := s.client.Object.Get(context.Background(), objectKey, nil)
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()
	return io.ReadAll(io.LimitReader(result.Body, 20*1024*1024+1))
}

func (s *COSStorage) DeleteObject(objectKey string) error {
	_, err := s.client.Object.Delete(context.Background(), objectKey)
	return err
}

func cosBucketURL(opts COSOptions) (*url.URL, error) {
	endpoint := strings.TrimSpace(opts.Endpoint)
	if endpoint != "" {
		if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
			endpoint = "https://" + endpoint
		}
		return url.Parse(endpoint)
	}
	return tencos.NewBucketURL(opts.Bucket, opts.Region, true)
}
