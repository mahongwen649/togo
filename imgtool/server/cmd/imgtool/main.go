package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"imgtool/server/internal/auth"
	"imgtool/server/internal/channels"
	"imgtool/server/internal/config"
	"imgtool/server/internal/db"
	"imgtool/server/internal/generation"
	"imgtool/server/internal/history"
	"imgtool/server/internal/httpapi"
	"imgtool/server/internal/retention"
	"imgtool/server/internal/secure"
	"imgtool/server/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	authService := auth.NewService(auth.NewSQLStore(database), auth.Options{
		AdminUsername: cfg.AdminUsername,
		AdminPassword: cfg.AdminPassword,
		SessionTTL:    24 * time.Hour,
	})
	if _, err := authService.BootstrapAdmin(); err != nil {
		log.Fatal(err)
	}
	channelService := channels.NewService(channels.NewSQLStore(database), secure.NewSecretBox(cfg.Secret))
	historyService := history.NewService(database)
	var imageStorage generation.Storage
	var fileSigner httpapi.FileSigner
	var fileReader httpapi.FileReader
	var fileDeleter httpapi.FileDeleter
	imageStorage, fileSigner, fileReader, fileDeleter, err = buildStorage(cfg)
	if err != nil {
		log.Fatal(err)
	}
	generationService := generation.NewService(channelService, historyService, generation.OpenAIProvider{
		InternalBaseURL: os.Getenv("IMGTOOL_INTERNAL_CORE_BASE_URL"),
	}, imageStorage)
	retention.Cleaner{
		History:   historyService,
		Deleter:   fileDeleter,
		Retention: retention.DefaultRetention,
	}.Start(context.Background(), 24*time.Hour, func(err error) {
		log.Printf("history retention cleanup failed: %v", err)
	})
	log.Printf("imgtool server listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, httpapi.NewServer(httpapi.Dependencies{
		Auth:          authService,
		Channels:      channelService,
		FileSigner:    fileSigner,
		FileReader:    fileReader,
		FileDeleter:   fileDeleter,
		Generation:    generationService,
		History:       historyService,
		SecureCookies: cfg.Env == "production",
		SSO: auth.SSOOptions{
			Secret:   cfg.SSOSecret,
			Issuer:   cfg.SSOIssuer,
			Audience: cfg.SSOAudience,
		},
	})); err != nil {
		log.Fatal(err)
	}
}

func buildStorage(cfg config.Config) (generation.Storage, httpapi.FileSigner, httpapi.FileReader, httpapi.FileDeleter, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.StorageProvider))
	switch provider {
	case "", "aliyun-oss":
		ossStorage, err := storage.NewOSSStorage(storage.OSSOptions{
			Bucket:           cfg.OSSBucket,
			Endpoint:         cfg.OSSEndpoint,
			Prefix:           cfg.OSSPrefix,
			AccessKeyID:      cfg.OSSAccessKeyID,
			AccessKeySecret:  cfg.OSSAccessKeySecret,
			SignedURLExpires: time.Duration(cfg.OSSSignedURLExpires) * time.Second,
		}, nil)
		if err != nil {
			return developmentStorage(cfg, err)
		}
		return ossStorage, ossStorage, ossStorage, ossStorage, nil
	case "tencent-cos":
		cosStorage, err := storage.NewCOSStorage(storage.COSOptions{
			Bucket:           cfg.COSBucket,
			Region:           cfg.COSRegion,
			Endpoint:         cfg.COSEndpoint,
			Prefix:           cfg.COSPrefix,
			SecretID:         cfg.COSSecretID,
			SecretKey:        cfg.COSSecretKey,
			SignedURLExpires: time.Duration(cfg.COSSignedURLExpires) * time.Second,
		}, nil)
		if err != nil {
			return developmentStorage(cfg, err)
		}
		return cosStorage, cosStorage, cosStorage, cosStorage, nil
	default:
		return nil, nil, nil, nil, fmt.Errorf("unsupported IMGTOOL_STORAGE_PROVIDER %q", cfg.StorageProvider)
	}
}

func developmentStorage(cfg config.Config, cause error) (generation.Storage, httpapi.FileSigner, httpapi.FileReader, httpapi.FileDeleter, error) {
	if cfg.Env == "production" {
		return nil, nil, nil, nil, cause
	}
	log.Printf("%s storage not configured; using local metadata storage for development", cfg.StorageProvider)
	localStorage := storage.NewLocalMetadataStorage(firstNonEmpty(cfg.COSBucket, cfg.OSSBucket), firstNonEmpty(cfg.COSPrefix, cfg.OSSPrefix), nil)
	return localStorage, nil, nil, localStorage, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
