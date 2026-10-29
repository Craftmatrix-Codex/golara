package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
	"os"
	"testing"
	"time"
)

func TestScopedRevocationKeepsOtherProjectsAndRejectsAmbiguousLegacy(t *testing.T) {
	dsn := os.Getenv("SUPADATA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SUPADATA_TEST_DATABASE_URL is not configured")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	schema := fmt.Sprintf("scope_auth_%d", time.Now().UnixNano())
	defer db.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+quoteIdentifier(schema)+` CASCADE`)
	if err := EnsurePostgresSchema(ctx, db, schema); err != nil {
		t.Fatal(err)
	}
	repo, err := NewPostgresRepository(db, schema)
	if err != nil {
		t.Fatal(err)
	}
	user, err := repo.CreateUser(ctx, "scope-test@example.invalid", "hash", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	alpha := project.WithScope(ctx, project.Project{ID: "alpha"})
	beta := project.WithScope(ctx, project.Project{ID: "beta"})
	create := func(c context.Context, name string) Session {
		t.Helper()
		session, err := repo.CreateSession(c, user.ID, name, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		return session
	}
	a1, a2, b1 := create(alpha, "alpha-1"), create(alpha, "alpha-2"), create(beta, "beta-1")
	if err := repo.RevokeSession(beta, a1.ID); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("local logout must reject another project's session")
	}

	active := func(session Session) bool {
		t.Helper()
		v, err := repo.IsSessionActive(ctx, session.ID)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if err := repo.RevokeUserSessions(alpha, user.ID, a1.ID, true); err != nil {
		t.Fatal(err)
	}
	if !active(a1) || active(a2) || !active(b1) {
		t.Fatal("others must only revoke the other alpha session")
	}
	a3 := create(alpha, "alpha-3")
	if err := repo.RevokeUserSessions(alpha, user.ID, a1.ID, false); err != nil {
		t.Fatal(err)
	}
	if active(a1) || active(a3) || !active(b1) {
		t.Fatal("global must revoke alpha only")
	}
	if err := repo.RevokeUserSessions(alpha, user.ID, a1.ID, false); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("revoked current session must not revoke other sessions")
	}
	legacy := create(beta, "legacy")
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`UPDATE %s.sessions SET golara_project_id=NULL WHERE id=$1`, quoteIdentifier(schema)), legacy.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.RevokeSession(beta, legacy.ID); !errors.Is(err, ErrUnscopedLegacySessions) {
		t.Fatalf("unscoped nondefault local logout must return an explicit conflict: %v", err)
	}
	if err := repo.RevokeUserSessions(beta, user.ID, legacy.ID, false); !errors.Is(err, ErrUnscopedLegacySessions) {
		t.Fatalf("unscoped nondefault global logout must return an explicit conflict: %v", err)
	}
	b2 := create(beta, "beta-2")
	if err := repo.RevokeUserSessions(beta, user.ID, b1.ID, false); err == nil {
		t.Fatal("ambiguous legacy session must fail closed")
	}
	if !active(legacy) || !active(b1) || !active(b2) {
		t.Fatal("failure must not revoke any session")
	}
}

func TestConcurrentLocalAndGlobalLogoutCannotReuseRevokedSession(t *testing.T) {
	dsn := os.Getenv("SUPADATA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SUPADATA_TEST_DATABASE_URL is not configured")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	schema := fmt.Sprintf("race_auth_%d", time.Now().UnixNano())
	defer db.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS `+quoteIdentifier(schema)+` CASCADE`)
	if err := EnsurePostgresSchema(ctx, db, schema); err != nil {
		t.Fatal(err)
	}
	repo, err := NewPostgresRepository(db, schema)
	if err != nil {
		t.Fatal(err)
	}
	user, err := repo.CreateUser(ctx, "race@example.invalid", "hash", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	session, err := repo.CreateSession(ctx, user.ID, "race-refresh", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback()
	var tokenID int64
	if err := blocker.QueryRowContext(ctx, fmt.Sprintf(`SELECT id FROM %s.refresh_tokens WHERE session_id=$1 FOR UPDATE`, quoteIdentifier(schema)), session.ID).Scan(&tokenID); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	go func() { results <- repo.RevokeSession(ctx, session.ID) }()
	waitForLocks := func(want int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			var waiting int
			if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity WHERE pid <> pg_backend_pid() AND wait_event_type='Lock' AND query LIKE $1`, "%"+schema+"%").Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if waiting >= want {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("expected %d blocked logout operations", want)
	}
	waitForLocks(1)
	go func() { results <- repo.RevokeUserSessions(ctx, user.ID, session.ID, false) }()
	waitForLocks(2)
	if err := blocker.Commit(); err != nil {
		t.Fatal(err)
	}
	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("exactly one logout must succeed: first=%v second=%v", first, second)
	}
	failure := first
	if failure == nil {
		failure = second
	}
	if !errors.Is(failure, ErrInvalidCredentials) {
		t.Fatalf("second logout error=%v", failure)
	}
}
