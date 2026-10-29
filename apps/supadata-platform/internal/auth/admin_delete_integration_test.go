package auth

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestAdminDeletionPersistsRequestedModeInPostgres(t *testing.T) {
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
	schema := fmt.Sprintf("delete_auth_%d", time.Now().UnixNano())
	defer db.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+quoteIdentifier(schema)+` CASCADE`)
	if err := EnsurePostgresSchema(ctx, db, schema); err != nil {
		t.Fatal(err)
	}
	repo, err := NewPostgresRepository(db, schema)
	if err != nil {
		t.Fatal(err)
	}
	for _, soft := range []bool{false, true} {
		user, err := repo.CreateUser(ctx, fmt.Sprintf("fixture-%v@example.invalid", soft), "hash", nil, true)
		if err != nil {
			t.Fatal(err)
		}
		session, err := repo.CreateSession(ctx, user.ID, fmt.Sprintf("refresh-%v", soft), time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if soft {
			err = repo.DeleteUserWithOptions(ctx, user.ID, true)
		} else {
			err = repo.DeleteUser(ctx, user.ID)
		}
		if err != nil {
			t.Fatal(err)
		}
		var users, sessions, tokens int
		for table, dest := range map[string]*int{"users": &users, "sessions": &sessions, "refresh_tokens": &tokens} {
			column := "user_id"
			if table == "users" {
				column = "id"
			}
			if err := db.QueryRowContext(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.%s WHERE %s = $1`, quoteIdentifier(schema), quoteIdentifier(table), column), user.ID).Scan(dest); err != nil {
				t.Fatal(err)
			}
		}
		expected := 0
		if soft {
			expected = 1
		}
		if users != expected || sessions != expected || tokens != expected {
			t.Fatalf("soft=%v users=%d sessions=%d tokens=%d", soft, users, sessions, tokens)
		}
		if _, _, err := repo.FindUserByEmail(ctx, user.Email); err == nil {
			t.Fatal("deleted user remains eligible for login")
		}
		if _, _, err := repo.RefreshSession(ctx, session.RefreshTokenHash, "new-refresh", time.Now().Add(time.Hour)); err == nil {
			t.Fatal("deleted user refreshed session")
		}
	}
}
