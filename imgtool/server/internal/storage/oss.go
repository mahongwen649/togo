package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"

	"imgtool/server/internal/generation"
)

type OSSOptions struct {
	Bucket           string
	Endpoint         string
	Region           string
	Prefix           string
	AccessKeyID      string
	AccessKeySecret  string
	SignedURLExpires time.Duration
}

type OSSStorage struct {
	client *oss.Client
	opts   OSSOptions
	now    func() time.Time
}

func NewOSSStorage(opts OSSOptions, now func() time.Time) (*OSSStorage, error) {
	if opts.Bucket == "" || opts.Endpoint == "" || opts.AccessKeyID == "" || opts.AccessKeySecret == "" {
		return nil, errors.New("oss storage is not configured")
	}
	if opts.Region == "" {
		opts.Region = regionFromEndpoint(opts.Endpoint)
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
	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.AccessKeySecret)).
		WithRegion(opts.Region).
		WithEndpoint(opts.Endpoint)
	return &OSSStorage{client: oss.NewClient(cfg), opts: opts, now: now}, nil
}

func (s *OSSStorage) SaveGeneratedImage(input generation.StoreImageInput) (generation.StoredImage, error) {
	mimeType := normalizeMime(input.MimeType)
	key := ObjectKey(s.opts.Prefix, s.now(), input)
	_, err := s.client.PutObject(context.Background(), &oss.PutObjectRequest{
		Bucket:      oss.Ptr(s.opts.Bucket),
		Key:         oss.Ptr(key),
		Body:        bytes.NewReader(input.Bytes),
		ContentType: oss.Ptr(mimeType),
	})
	if err != nil {
		return generation.StoredImage{}, err
	}
	return generation.StoredImage{
		StorageProvider: "aliyun-oss",
		Bucket:          s.opts.Bucket,
		ObjectKey:       key,
		Role:            normalizedRole(input.Role),
		MediaType:       "image",
		MimeType:        mimeType,
		SizeBytes:       int64(len(input.Bytes)),
	}, nil
}

func normalizedRole(value string) string {
	if value == "reference" {
		return "reference"
	}
	return "result"
}

func (s *OSSStorage) SignedGetURL(objectKey string) (string, error) {
	result, err := s.client.Presign(context.Background(), &oss.GetObjectRequest{
		Bucket: oss.Ptr(s.opts.Bucket),
		Key:    oss.Ptr(objectKey),
	}, oss.PresignExpires(s.opts.SignedURLExpires))
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (s *OSSStorage) SignedDownloadURL(objectKey string, filename string) (string, error) {
	filename = strings.TrimSpace(strings.ReplaceAll(filename, `"`, ""))
	if filename == "" {
		filename = "image.png"
	}
	disposition := fmt.Sprintf(`attachment; filename="%s"`, filename)
	result, err := s.client.Presign(context.Background(), &oss.GetObjectRequest{
		Bucket:                     oss.Ptr(s.opts.Bucket),
		Key:                        oss.Ptr(objectKey),
		ResponseContentDisposition: oss.Ptr(disposition),
	}, oss.PresignExpires(s.opts.SignedURLExpires))
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (s *OSSStorage) ReadObject(objectKey string) ([]byte, error) {
	result, err := s.client.GetObject(context.Background(), &oss.GetObjectRequest{
		Bucket: oss.Ptr(s.opts.Bucket),
		Key:    oss.Ptr(objectKey),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()
	return io.ReadAll(io.LimitReader(result.Body, 20*1024*1024+1))
}

func (s *OSSStorage) DeleteObject(objectKey string) error {
	_, err := s.client.DeleteObject(context.Background(), &oss.DeleteObjectRequest{
		Bucket: oss.Ptr(s.opts.Bucket),
		Key:    oss.Ptr(objectKey),
	})
	return err
}

func regionFromEndpoint(endpoint string) string {
	switch {
	case endpoint == "https://oss-cn-guangzhou.aliyuncs.com" || endpoint == "http://oss-cn-guangzhou.aliyuncs.com":
		return "cn-guangzhou"
	default:
		return "cn-guangzhou"
	}
}
