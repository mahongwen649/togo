package config

import (
	"os"
	"testing"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("IMGTOOL_HTTP_ADDR", "")
	t.Setenv("IMGTOOL_DATABASE_PATH", "")
	t.Setenv("IMGTOOL_ENV", "")
	t.Setenv("IMGTOOL_SECRET", "test-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.DatabasePath != "./data/imgtool.sqlite" {
		t.Fatalf("DatabasePath = %q", cfg.DatabasePath)
	}
	if cfg.Env != "development" {
		t.Fatalf("Env = %q", cfg.Env)
	}
}

func TestLoadRequiresSecret(t *testing.T) {
	t.Setenv("IMGTOOL_SECRET", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected missing secret error")
	}
}

func TestLoadDotEnvFile(t *testing.T) {
	dir := t.TempDir()
	dotenv := dir + "/.env"
	if err := os.WriteFile(dotenv, []byte("IMGTOOL_SECRET=from-env-file\nALIYUN_OSS_BUCKET=whalesing-web\nTENCENT_COS_BUCKET=togoapi-img-1385779910\n"), 0o600); err != nil {
		t.Fatalf("write dotenv: %v", err)
	}
	t.Setenv("IMGTOOL_SECRET", "")
	t.Setenv("ALIYUN_OSS_BUCKET", "")
	t.Setenv("TENCENT_COS_BUCKET", "")

	cfg, err := LoadFromFile(dotenv)
	if err != nil {
		t.Fatalf("LoadFromFile returned error: %v", err)
	}
	if cfg.Secret != "from-env-file" || cfg.OSSBucket != "whalesing-web" || cfg.COSBucket != "togoapi-img-1385779910" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestLoadTencentCOSConfig(t *testing.T) {
	t.Setenv("IMGTOOL_SECRET", "test-secret")
	t.Setenv("IMGTOOL_STORAGE_PROVIDER", "tencent-cos")
	t.Setenv("TENCENT_COS_BUCKET", "togoapi-img-1385779910")
	t.Setenv("TENCENT_COS_REGION", "ap-guangzhou")
	t.Setenv("TENCENT_COS_ENDPOINT", "https://togoapi-img-1385779910.cos.ap-guangzhou.myqcloud.com")
	t.Setenv("TENCENT_COS_PREFIX", "customPrefix")
	t.Setenv("TENCENT_COS_SIGNED_URL_EXPIRES", "900")
	t.Setenv("TENCENT_COS_SECRET_ID", "secret-id")
	t.Setenv("TENCENT_COS_SECRET_KEY", "secret-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.StorageProvider != "tencent-cos" || cfg.COSBucket != "togoapi-img-1385779910" || cfg.COSRegion != "ap-guangzhou" {
		t.Fatalf("unexpected cos config: %#v", cfg)
	}
	if cfg.COSPrefix != "customPrefix" || cfg.COSSignedURLExpires != 900 {
		t.Fatalf("unexpected cos defaults: %#v", cfg)
	}
	if cfg.COSSecretID != "secret-id" || cfg.COSSecretKey != "secret-key" {
		t.Fatalf("unexpected cos credentials")
	}
}
