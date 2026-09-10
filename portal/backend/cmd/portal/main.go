package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	portalauth "github.com/jqcode/portal/backend/internal/auth"
	"github.com/jqcode/portal/backend/internal/balancealert"
	"github.com/jqcode/portal/backend/internal/company"
	"github.com/jqcode/portal/backend/internal/coreclient"
	"github.com/jqcode/portal/backend/internal/httpapi"
	"github.com/jqcode/portal/backend/internal/lottery"
	"github.com/jqcode/portal/backend/internal/passwordreset"
	"github.com/jqcode/portal/backend/internal/rechargepromo"
	"github.com/jqcode/portal/backend/internal/storage"
	"github.com/jqcode/portal/backend/internal/username"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	coreBaseURL := envOrDefault("CORE_BASE_URL", "http://127.0.0.1:18080")
	listenAddress := envOrDefault("PORTAL_LISTEN_ADDR", "127.0.0.1:18081")

	adapter, err := coreclient.NewHTTPAdapter(coreBaseURL, &http.Client{Timeout: 10 * time.Second})
	if err != nil {
		logger.Error("invalid Core configuration", "error", err)
		os.Exit(1)
	}

	var authService httpapi.AuthService
	var companyService *company.Service
	var usernameRegistry *username.Registry
	var passwordResetService httpapi.PasswordResetService
	var corePaymentDB any
	var rechargeCampaign any
	var lotteryService *lottery.Service
	var portalDB *sql.DB
	databaseURL := os.Getenv("PORTAL_DATABASE_URL")
	adminAPIKey := os.Getenv("CORE_ADMIN_API_KEY")
	if databaseURL != "" {
		if adminAPIKey == "" {
			logger.Error("username authentication requires CORE_ADMIN_API_KEY when PORTAL_DATABASE_URL is configured")
			os.Exit(1)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		db, err := storage.Open(ctx, databaseURL)
		if err != nil {
			logger.Error("Portal database initialization failed", "error", err)
			os.Exit(1)
		}
		defer db.Close()
		portalDB = db
		if err := storage.Migrate(ctx, db); err != nil {
			logger.Error("Portal database migration failed", "error", err)
			os.Exit(1)
		}
		registry := username.New(storage.NewUsernameStore(db))
		usernameRegistry = registry
		authService = portalauth.New(adapter.WithAdminAPIKey(adminAPIKey), registry)
		companyService = company.New(db)
		rechargeCampaign = rechargepromo.New(db, adapter.WithAdminAPIKey(adminAPIKey))
		lotteryService = lottery.New(db, adapter.WithAdminAPIKey(adminAPIKey), adapter.WithAdminAPIKey(adminAPIKey), logger)
		go lotteryService.RunWorker(context.Background(), time.Duration(envInt("PORTAL_LOTTERY_WORKER_INTERVAL_SECONDS", 15))*time.Second)

		balanceAlertEnabled := envBool("PORTAL_BALANCE_ALERT_ENABLED", false)
		passwordResetEnabled := os.Getenv("PORTAL_PASSWORD_RESET_SECRET") != ""
		var mailer *passwordreset.SMTPMailer
		if passwordResetEnabled || balanceAlertEnabled {
			mailer, err = passwordreset.NewSMTPMailer(passwordreset.SMTPConfig{
				Host: os.Getenv("PORTAL_SMTP_HOST"), Port: envInt("PORTAL_SMTP_PORT", 465),
				Username: os.Getenv("PORTAL_SMTP_USERNAME"), Password: os.Getenv("PORTAL_SMTP_PASSWORD"),
				From: os.Getenv("PORTAL_SMTP_FROM"), FromName: envOrDefault("PORTAL_SMTP_FROM_NAME", "TogoAPI"),
				TLS: envBool("PORTAL_SMTP_USE_TLS", true),
			})
			if err != nil {
				logger.Error("Portal password reset SMTP initialization failed", "error", err)
				os.Exit(1)
			}
		}
		if passwordResetEnabled {
			var verifier passwordreset.HumanVerifier = passwordreset.DisabledTurnstileVerifier{}
			if secret := os.Getenv("PORTAL_TURNSTILE_SECRET"); secret != "" {
				verifier, err = passwordreset.NewTurnstileVerifier(secret, &http.Client{Timeout: 10 * time.Second})
				if err != nil {
					logger.Error("Portal password reset Turnstile initialization failed", "error", err)
					os.Exit(1)
				}
			}
			passwordResetService, err = passwordreset.New(db, adapter.WithAdminAPIKey(adminAPIKey), mailer, verifier, []byte(os.Getenv("PORTAL_PASSWORD_RESET_SECRET")))
			if err != nil {
				logger.Error("Portal password reset initialization failed", "error", err)
				os.Exit(1)
			}
		} else {
			logger.Warn("Portal verification-code password reset is not configured")
		}
		if balanceAlertEnabled {
			monitor, err := balancealert.New(
				adapter.WithAdminAPIKey(adminAPIKey),
				balancealert.NewSQLStore(portalDB),
				mailer,
				logger,
				balancealert.Config{Interval: time.Duration(envInt("PORTAL_BALANCE_ALERT_INTERVAL_SECONDS", 60)) * time.Second},
			)
			if err != nil {
				logger.Error("Portal balance alert initialization failed", "error", err)
				os.Exit(1)
			}
			go monitor.Run(context.Background())
		} else {
			logger.Warn("Portal balance alert monitor is not enabled")
		}
	} else if adminAPIKey != "" {
		logger.Warn("Portal database is not configured; using direct Core email login and disabling database-backed Portal features")
	}
	if coreDatabaseURL := os.Getenv("CORE_DATABASE_URL"); coreDatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		db, err := storage.Open(ctx, coreDatabaseURL)
		if err != nil {
			logger.Error("Core payment database initialization failed", "error", err)
			os.Exit(1)
		}
		defer db.Close()
		corePaymentDB = db
	}

	server := &http.Server{
		Addr:              listenAddress,
		Handler:           httpapi.New(adapter.WithAdminAPIKey(adminAPIKey), authService, logger, companyService, usernameRegistry, passwordResetService, rechargeCampaign, lotteryService, corePaymentDB),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	logger.Info("Portal API listening", "address", listenAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Portal API stopped", "error", err)
		os.Exit(1)
	}
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
