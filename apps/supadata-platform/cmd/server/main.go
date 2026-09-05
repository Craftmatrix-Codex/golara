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

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/auth"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/config"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/httpapi"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/registry"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/rest"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/storage"
)

func main() {
	cfg := config.Load()
	store, err := registry.New(registry.Options{DataDir: cfg.DataDir})
	if err != nil {
		slog.Error("initialize registry", "error", err)
		os.Exit(1)
	}

	var database *sql.DB
	var authService httpapi.AuthService
	var restHandler http.Handler
	var storageHandler http.Handler
	if cfg.DatabaseURL != "" {
		database, err = sql.Open("pgx", cfg.DatabaseURL)
		if err != nil {
			slog.Error("open PostgreSQL", "error", err)
			os.Exit(1)
		}
		defer database.Close()
		pingContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = database.PingContext(pingContext)
		cancel()
		if err != nil {
			slog.Error("connect to PostgreSQL", "error", err)
			os.Exit(1)
		}
		repository, repositoryErr := auth.NewPostgresRepository(database, "auth")
		if repositoryErr != nil {
			slog.Error("initialize Auth repository", "error", repositoryErr)
			os.Exit(1)
		}
		authService = auth.NewService(repository, auth.ServiceOptions{
			JWTSecret:   []byte(cfg.JWTSecret),
			Issuer:      cfg.AuthIssuer,
			Audience:    "authenticated",
			TokenTTL:    time.Hour,
			AutoConfirm: cfg.AuthAutoConfirm,
		})
		restHandler = rest.NewHandler(database, rest.HandlerOptions{APIKeys: rest.APIKeyConfig{Anon: cfg.AnonKey, ServiceRole: cfg.ServiceRoleKey}, JWTSecret: []byte(cfg.JWTSecret), Issuer: cfg.AuthIssuer, Audience: "authenticated"})
	} else {
		slog.Warn("PostgreSQL is not configured; Auth routes are unavailable")
	}
	if cfg.StorageEndpoint != "" || cfg.StorageAccessKey != "" || cfg.StorageSecretKey != "" {
		objectStore, storageErr := storage.NewS3Store(storage.S3Config{
			Endpoint:  cfg.StorageEndpoint,
			AccessKey: cfg.StorageAccessKey,
			SecretKey: cfg.StorageSecretKey,
			Region:    cfg.StorageRegion,
			UseSSL:    cfg.StorageUseSSL,
		})
		if storageErr != nil {
			slog.Error("initialize object storage", "error", storageErr)
			os.Exit(1)
		}
		storageHandler = storage.NewHandler(storage.HandlerOptions{
			Store:     objectStore,
			APIKeys:   storage.APIKeyConfig{Anon: cfg.AnonKey, ServiceRole: cfg.ServiceRoleKey},
			JWTSecret: []byte(cfg.JWTSecret),
			Issuer:    cfg.AuthIssuer,
			Audience:  "authenticated",
		})
	}

	server := &http.Server{
		Addr: "0.0.0.0:" + formatPort(cfg.Port),
		Handler: httpapi.NewServer(httpapi.ServerOptions{
			Token:         cfg.ControlPlaneToken,
			AllowedOrigin: cfg.AllowedOrigin,
			Registry:      store,
			Auth:          authService,
			APIKeys:       httpapi.APIKeyConfig{Anon: cfg.AnonKey, ServiceRole: cfg.ServiceRoleKey},
			AuthSettings:  httpapi.AuthSettings{EmailEnabled: cfg.AuthEmailEnabled, PhoneEnabled: cfg.AuthPhoneEnabled, MailerAutoconfirm: cfg.AuthAutoConfirm, SMSProvider: cfg.SMSProvider, DisableSignup: cfg.AuthDisableSignup},
			REST:          restHandler,
			Storage:       storageHandler,
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-shutdownContext.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			slog.Error("graceful shutdown", "error", err)
		}
	}()

	slog.Info("Supadata Go platform listening", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func formatPort(port int) string {
	if port < 1 {
		return "8090"
	}
	return strconv.Itoa(port)
}
