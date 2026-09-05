package auth

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresRepositoryPasswordSessionRoundTrip(t *testing.T) {
	databaseURL := os.Getenv("SUPADATA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SUPADATA_TEST_DATABASE_URL is not configured")
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	_, err = database.ExecContext(ctx, `
		DROP SCHEMA IF EXISTS auth CASCADE;
		CREATE SCHEMA auth;
		CREATE TABLE auth.users (
			id uuid PRIMARY KEY, email varchar(255) UNIQUE, aud varchar(255), role varchar(255),
			encrypted_password varchar(255), raw_app_meta_data jsonb, raw_user_meta_data jsonb,
			created_at timestamptz, updated_at timestamptz, confirmed_at timestamptz, deleted_at timestamptz
		);
		CREATE TABLE auth.sessions (id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES auth.users(id), created_at timestamptz, updated_at timestamptz, not_after timestamptz, refreshed_at timestamptz);
		CREATE TABLE auth.refresh_tokens (id bigserial PRIMARY KEY, token varchar(255), user_id varchar(255), revoked boolean, created_at timestamptz, updated_at timestamptz, session_id uuid REFERENCES auth.sessions(id));
	`)
	if err != nil {
		t.Fatalf("create auth fixture: %v", err)
	}

	repository, err := NewPostgresRepository(database, "auth")
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	user, err := repository.CreateUser(ctx, "user@example.com", "bcrypt-hash", map[string]any{"provider": "email"}, true)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	found, passwordHash, err := repository.FindUserByEmail(ctx, "USER@example.com")
	if err != nil || found.ID != user.ID || passwordHash != "bcrypt-hash" {
		t.Fatalf("find user = %+v, hash=%q, err=%v", found, passwordHash, err)
	}
	session, err := repository.CreateSession(ctx, user.ID, "old-refresh-hash", time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	rotatedUser, rotatedSession, err := repository.RefreshSession(ctx, "old-refresh-hash", "new-refresh-hash", time.Now().UTC().Add(2*time.Hour))
	if err != nil || rotatedUser.ID != user.ID || rotatedSession.ID != session.ID {
		t.Fatalf("refresh = user=%+v session=%+v err=%v", rotatedUser, rotatedSession, err)
	}
	var revoked bool
	if err := database.QueryRowContext(ctx, `SELECT revoked FROM auth.refresh_tokens WHERE token = 'old-refresh-hash'`).Scan(&revoked); err != nil || !revoked {
		t.Fatalf("old refresh token revoked=%v err=%v", revoked, err)
	}
	if err := repository.RevokeSession(ctx, session.ID); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	var activeRefreshTokens int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM auth.refresh_tokens WHERE session_id = $1 AND revoked = false`, session.ID).Scan(&activeRefreshTokens); err != nil || activeRefreshTokens != 0 {
		t.Fatalf("active refresh tokens=%d err=%v", activeRefreshTokens, err)
	}
}
