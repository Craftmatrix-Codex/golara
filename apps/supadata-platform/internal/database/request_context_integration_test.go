package database

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

func TestRequestContextIsScopedAndResetOnReusedPostgresConnection(t *testing.T) {
	dsn := os.Getenv("SUPADATA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SUPADATA_TEST_DATABASE_URL is not configured")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	var originalRole string
	if err := db.QueryRow("SELECT current_user").Scan(&originalRole); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		role, subject, project string
		rollback               bool
	}{{"authenticated", "user-a", "alpha", false}, {"anon", "", "beta", false}, {"service_role", "", "alpha", false}, {"authenticated", "user-c", "beta", true}} {
		ctx := project.WithScope(context.Background(), project.Project{ID: tc.project})
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := SetRequestContext(ctx, tx, jwt.Claims{Role: tc.role, Subject: tc.subject}, "anon"); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		var role, subject, claimRole, projectID, jsonProject string
		err = tx.QueryRowContext(ctx, `SELECT current_user,current_setting('request.jwt.claim.sub'),current_setting('request.jwt.claim.role'),current_setting('request.jwt.claim.project_id'),current_setting('request.jwt.claims')::jsonb->>'project_id'`).Scan(&role, &subject, &claimRole, &projectID, &jsonProject)
		if err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if role != tc.role || subject != tc.subject || claimRole != tc.role || projectID != tc.project || jsonProject != tc.project {
			tx.Rollback()
			t.Fatalf("unexpected role/claims: %s %s %s %s %s", role, subject, claimRole, projectID, jsonProject)
		}
		if tc.rollback {
			err = tx.Rollback()
		} else {
			err = tx.Commit()
		}
		if err != nil {
			t.Fatal(err)
		}
		var outsideRole, outsideSubject, outsideJSON, outsideClaimRole, outsideProject string
		if err := db.QueryRow(`SELECT current_user,
			coalesce(current_setting('request.jwt.claim.sub',true),''),
			coalesce(current_setting('request.jwt.claims',true),''),
			coalesce(current_setting('request.jwt.claim.role',true),''),
			coalesce(current_setting('request.jwt.claim.project_id',true),'')`).Scan(&outsideRole, &outsideSubject, &outsideJSON, &outsideClaimRole, &outsideProject); err != nil {
			t.Fatal(err)
		}
		if outsideRole != originalRole || outsideSubject != "" || outsideJSON != "" || outsideClaimRole != "" || outsideProject != "" {
			t.Fatalf("transaction context leaked: role=%s subject=%s json=%s claimRole=%s project=%s", outsideRole, outsideSubject, outsideJSON, outsideClaimRole, outsideProject)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := SetRequestContext(context.Background(), tx, jwt.Claims{Role: "postgres"}, "anon"); err == nil {
		t.Fatal("privileged database role was accepted")
	}
	ctx := project.WithScope(context.Background(), project.Project{ID: "beta"})
	if err := SetRequestContext(ctx, tx, jwt.Claims{Role: "authenticated", ProjectID: "alpha"}, "anon"); err == nil {
		t.Fatal("cross-project claims were accepted")
	}
}
