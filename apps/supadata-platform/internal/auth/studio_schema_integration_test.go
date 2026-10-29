package auth

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestAuthSchemaSupportsStudioUserListing(t *testing.T) {
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
	schema := fmt.Sprintf("studio_auth_%d", time.Now().UnixNano())
	defer db.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+quoteIdentifier(schema)+` CASCADE`)
	for i := 0; i < 2; i++ {
		if err := EnsurePostgresSchema(ctx, db, schema); err != nil {
			t.Fatal(err)
		}
	}
	repo, err := NewPostgresRepository(db, schema)
	if err != nil {
		t.Fatal(err)
	}
	user, err := repo.CreateUser(ctx, "studio-fixture@example.com", "hash", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	query := fmt.Sprintf(`SELECT u.email,u.banned_until,u.confirmation_sent_at,u.is_anonymous,u.is_sso_user,u.invited_at,u.last_sign_in_at,u.phone,u.confirmed_at,u.email_confirmed_at,u.phone_confirmed_at,coalesce((SELECT array_agg(DISTINCT i.provider) FROM %s.identities i WHERE i.user_id=u.id),'{}'::text[]) AS providers FROM %s.users u WHERE u.id=$1`, quoteIdentifier(schema), quoteIdentifier(schema))
	rows, err := db.QueryContext(ctx, query, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("Studio could not list a real Auth user")
	}
}
