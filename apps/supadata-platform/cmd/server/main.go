package main

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/database"
	graphqlapi "github.com/renzaspiras/supabase/apps/supadata-platform/internal/graphql"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/httpapi"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/provisioning"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/realtime"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/registry"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/rest"
	platformruntime "github.com/renzaspiras/supabase/apps/supadata-platform/internal/runtime"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/storage"
)

type bucketProvisioner interface {
	EnsureBucket(context.Context, string) error
}

func ensureProjectBuckets(ctx context.Context, provisioner bucketProvisioner, projects []project.Project) error {
	for _, configuredProject := range projects {
		bucket := configuredProject.Scope.Storage.Bucket
		if bucket == "" {
			return fmt.Errorf("project %s has no configured storage bucket", configuredProject.ID)
		}
		if err := provisioner.EnsureBucket(ctx, bucket); err != nil {
			return fmt.Errorf("project %s storage bucket %s: %w", configuredProject.ID, bucket, err)
		}
	}
	return nil
}

func main() {
	cfg := config.Load()
	store, err := registry.New(registry.Options{DataDir: cfg.DataDir, PublicHost: cfg.PublicHost})
	if err != nil {
		slog.Error("initialize registry", "error", err)
		os.Exit(1)
	}
	databaseHost, databasePort, databaseName, databaseUser, connectionString := cfg.PublicDatabaseDetails()
	if err := store.SetPublicDatabaseMetadata(databaseHost, databasePort, databaseName, databaseUser, connectionString); err != nil {
		slog.Error("initialize public database metadata", "error", err)
		os.Exit(1)
	}

	projects, err := store.ListProjects(context.Background())
	if err != nil {
		slog.Error("read project registry", "error", err)
		os.Exit(1)
	}
	if len(projects) == 0 {
		if _, err := store.CreateProject(context.Background(), "Default Project", "default"); err != nil {
			slog.Error("bootstrap default project", "error", err)
			os.Exit(1)
		}
		projects, err = store.ListProjects(context.Background())
		if err != nil {
			slog.Error("read bootstrapped project registry", "error", err)
			os.Exit(1)
		}
		slog.Info("bootstrapped default project", "project", "default")
	}
	databaseConnections, err := platformruntime.OpenProjectDatabases(context.Background(), cfg, projects)
	if err != nil {
		slog.Error("initialize project databases", "error", err)
		os.Exit(1)
	}
	if databaseConnections != nil {
		defer databaseConnections.Close()
	}

	var authService httpapi.AuthService
	var restHandler http.Handler
	var graphqlHandler http.Handler
	var storageHandler http.Handler
	var realtimeHandler http.Handler
	var objectStore *storage.S3Store
	if databaseConnections != nil && databaseConnections.Primary != nil {
		if schemaErr := auth.EnsurePostgresSchema(context.Background(), databaseConnections.Primary, "auth"); schemaErr != nil {
			slog.Error("reconcile Auth schema", "error", schemaErr)
			os.Exit(1)
		}
		repository, repositoryErr := auth.NewPostgresRepository(databaseConnections.Primary, "auth")
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
		restHandler = rest.NewHandler(databaseConnections.Primary, rest.HandlerOptions{APIKeys: rest.APIKeyConfig{Anon: cfg.AnonKey, ServiceRole: cfg.ServiceRoleKey}, JWTSecret: []byte(cfg.JWTSecret), Issuer: cfg.AuthIssuer, Audience: "authenticated"})
		graphqlHandler = graphqlapi.NewHandler(graphqlapi.HandlerOptions{Database: databaseConnections.Primary, APIKeys: graphqlapi.APIKeyConfig{Anon: cfg.AnonKey, ServiceRole: cfg.ServiceRoleKey}, JWTSecret: []byte(cfg.JWTSecret), Issuer: cfg.AuthIssuer, Audience: "authenticated"})
	} else {
		slog.Warn("PostgreSQL is not configured; Auth routes are unavailable")
	}
	if graphqlHandler == nil {
		graphqlHandler = graphqlapi.NewHandler(graphqlapi.HandlerOptions{APIKeys: graphqlapi.APIKeyConfig{Anon: cfg.AnonKey, ServiceRole: cfg.ServiceRoleKey}, JWTSecret: []byte(cfg.JWTSecret), Issuer: cfg.AuthIssuer, Audience: "authenticated"})
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
		if storageErr := ensureProjectBuckets(context.Background(), objectStore, projects); storageErr != nil {
			slog.Error("initialize project storage", "error", storageErr)
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
	if cfg.DatabaseMode == "shared" && databaseConnections != nil && databaseConnections.Primary != nil && objectStore != nil {
		store.SetProvisioner(provisioning.Composite{
			Database: database.SharedProvisioner{DB: databaseConnections.Primary, Router: databaseConnections.Router},
			Storage:  objectStore,
		})
	}
	realtimeHandler = realtime.NewHandler(realtime.HandlerOptions{
		APIKeys:       realtime.APIKeyConfig{Anon: cfg.AnonKey, ServiceRole: cfg.ServiceRoleKey},
		JWTSecret:     []byte(cfg.JWTSecret),
		Issuer:        cfg.AuthIssuer,
		Audience:      "authenticated",
		AllowedOrigin: cfg.AllowedOrigin,
	})

	server := &http.Server{
		Addr: "0.0.0.0:" + formatPort(cfg.Port),
		Handler: httpapi.NewServer(httpapi.ServerOptions{
			Token:                cfg.ControlPlaneToken,
			ControlPlaneUsername: cfg.StudioAuthUsername,
			ControlPlanePassword: cfg.StudioAuthPassword,
			AllowedOrigin:        cfg.AllowedOrigin,
			Registry:             store,
			ProjectResolver:      store,
			DatabaseResolver:     databaseResolver(databaseConnections),
			RequireProjectScope:  cfg.RequireProjectScope,
			Auth:                 authService,
			APIKeys:              httpapi.APIKeyConfig{Anon: cfg.AnonKey, ServiceRole: cfg.ServiceRoleKey},
			AuthSettings:         httpapi.AuthSettings{EmailEnabled: cfg.AuthEmailEnabled, PhoneEnabled: cfg.AuthPhoneEnabled, MailerAutoconfirm: cfg.AuthAutoConfirm, SMSProvider: cfg.SMSProvider, DisableSignup: cfg.AuthDisableSignup},
			REST:                 restHandler,
			GraphQL:              graphqlHandler,
			Storage:              storageHandler,
			Realtime:             realtimeHandler,
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

func databaseResolver(connections *platformruntime.DatabaseConnections) database.Resolver {
	if connections == nil {
		return nil
	}
	return connections.Router
}

func formatPort(port int) string {
	if port < 1 {
		return "8090"
	}
	return strconv.Itoa(port)
}
