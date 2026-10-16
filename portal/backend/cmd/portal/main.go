package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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
	appContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
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
		go lotteryService.RunWorker(appContext, time.Duration(envInt("PORTAL_LOTTERY_WORKER_INTERVAL_SECONDS", 15))*time.Second)
		go httpapi.RunAccountRenewalWorker(appContext, db, adapter.WithAdminAPIKey(adminAPIKey), logger, time.Duration(envInt("PORTAL_ACCOUNT_RENEWAL_INTERVAL_SECONDS", 300))*time.Second)

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
			go monitor.Run(appContext)
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

	var providerID int64
	if raw := os.Getenv("PORTAL_ACCOUNT_PAYMENT_PROVIDER_ID"); raw != "" {
		var err error
		providerID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || providerID < 0 {
			logger.Error("invalid account payment provider ID")
			os.Exit(1)
		}
	}
	accountPayment := httpapi.AccountPaymentConfig{PublicURL: os.Getenv("PORTAL_PUBLIC_URL"), Secret: os.Getenv("PORTAL_ACCOUNT_PAYMENT_SECRET"), ProviderID: providerID}
	go httpapi.RunAccountPaymentWorker(appContext, portalDB, logger, accountPayment)
	exclusive := httpapi.ExclusiveConfig{Enabled: envBool("PORTAL_EXCLUSIVE_ENABLED", false), IngressIsolated: envBool("PORTAL_EXCLUSIVE_INGRESS_ISOLATED", false), PublicURL: os.Getenv("PORTAL_EXCLUSIVE_PUBLIC_URL"), Secret: os.Getenv("PORTAL_EXCLUSIVE_SECRET"), Context: appContext}
	if err := exclusive.Validate(); err != nil || (exclusive.Enabled && portalDB == nil) {
		logger.Error("invalid exclusive channel configuration", "error", err)
		os.Exit(1)
	}
	pelicanRunner := httpapi.NewPelicanRunner(portalDB, adapter.WithAdminAPIKey(adminAPIKey), logger)
	if portalDB != nil {
		go pelicanRunner.Run(appContext)
	}
	server := &http.Server{
		Addr:              listenAddress,
		Handler:           httpapi.New(adapter.WithAdminAPIKey(adminAPIKey), authService, logger, companyService, usernameRegistry, passwordResetService, rechargeCampaign, lotteryService, corePaymentDB, httpapi.NewPortalDatabase(portalDB), accountPayment, exclusive, pelicanRunner),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	logger.Info("Portal API listening", "address", listenAddress)
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.ListenAndServe() }()
	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Portal API stopped", "error", err)
			os.Exit(1)
		}
	case <-appContext.Done():
		shutdownTimeout := time.Duration(envInt("PORTAL_SHUTDOWN_TIMEOUT_SECONDS", 1200)) * time.Second
		logger.Info("Portal API draining active requests", "timeout", shutdownTimeout)
		shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		err := server.Shutdown(shutdownContext)
		if drainErr := pelicanRunner.Shutdown(shutdownContext); drainErr != nil {
			logger.Error("pelican shutdown drain timed out", "error", drainErr)
		}
		cancel()
		if err != nil {
			logger.Error("Portal API graceful shutdown timed out", "error", err)
			os.Exit(1)
		}
		if err = <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Portal API stopped", "error", err)
			os.Exit(1)
		}
		logger.Info("Portal API shutdown complete")
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
