package config

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Env                 string
	HTTPAddr            string
	DatabasePath        string
	Secret              string
	AdminUsername       string
	AdminPassword       string
	StorageProvider     string
	OSSBucket           string
	OSSEndpoint         string
	OSSPrefix           string
	OSSSignedURLExpires int
	OSSAccessKeyID      string
	OSSAccessKeySecret  string
	COSBucket           string
	COSRegion           string
	COSEndpoint         string
	COSPrefix           string
	COSSignedURLExpires int
	COSSecretID         string
	COSSecretKey        string
	SSOSecret           string
	SSOIssuer           string
	SSOAudience         string
}

func Load() (Config, error) {
	_ = loadDotEnv(".env")
	_ = loadDotEnv("../.env")
	return loadFromEnv()
}

func LoadFromFile(path string) (Config, error) {
	if err := loadDotEnv(path); err != nil {
		return Config{}, err
	}
	return loadFromEnv()
}

func loadFromEnv() (Config, error) {
	cfg := Config{
		Env:                 getenv("IMGTOOL_ENV", "development"),
		HTTPAddr:            getenv("IMGTOOL_HTTP_ADDR", "127.0.0.1:8080"),
		DatabasePath:        getenv("IMGTOOL_DATABASE_PATH", "./data/imgtool.sqlite"),
		Secret:              os.Getenv("IMGTOOL_SECRET"),
		AdminUsername:       getenv("IMGTOOL_ADMIN_USERNAME", "admin"),
		AdminPassword:       os.Getenv("IMGTOOL_ADMIN_PASSWORD"),
		StorageProvider:     getenv("IMGTOOL_STORAGE_PROVIDER", "aliyun-oss"),
		OSSBucket:           os.Getenv("ALIYUN_OSS_BUCKET"),
		OSSEndpoint:         os.Getenv("ALIYUN_OSS_ENDPOINT"),
		OSSPrefix:           getenv("ALIYUN_OSS_PREFIX", "aiImg"),
		OSSSignedURLExpires: getenvInt("ALIYUN_OSS_SIGNED_URL_EXPIRES", 600),
		OSSAccessKeyID:      os.Getenv("ALIYUN_OSS_ACCESS_KEY_ID"),
		OSSAccessKeySecret:  os.Getenv("ALIYUN_OSS_ACCESS_KEY_SECRET"),
		COSBucket:           os.Getenv("TENCENT_COS_BUCKET"),
		COSRegion:           os.Getenv("TENCENT_COS_REGION"),
		COSEndpoint:         os.Getenv("TENCENT_COS_ENDPOINT"),
		COSPrefix:           getenv("TENCENT_COS_PREFIX", "aiImg"),
		COSSignedURLExpires: getenvInt("TENCENT_COS_SIGNED_URL_EXPIRES", 600),
		COSSecretID:         os.Getenv("TENCENT_COS_SECRET_ID"),
		COSSecretKey:        os.Getenv("TENCENT_COS_SECRET_KEY"),
		SSOSecret:           os.Getenv("IMGTOOL_SSO_SECRET"),
		SSOIssuer:           getenv("IMGTOOL_SSO_ISSUER", "sub2api"),
		SSOAudience:         getenv("IMGTOOL_SSO_AUDIENCE", "imgtool"),
	}
	if cfg.Secret == "" {
		return Config{}, errors.New("IMGTOOL_SECRET is required")
	}
	return cfg, nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
