package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/auth"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/config"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestAuthSchemaReconcilesEveryProjectDatabase(t *testing.T) {
	raw := os.Getenv("SUPADATA_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("SUPADATA_TEST_DATABASE_URL is not configured")
	}
	base, err := url.Parse(raw)
	if err != nil || base.Hostname() != "golara-sdk-pg" || base.Path != "/golara_sdk" {
		t.Skip("requires the fixed disposable local database harness")
	}
	admin, err := sql.Open("pgx", raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	prefix := fmt.Sprintf("golara_scope_%d", time.Now().UnixNano())
	var created []string
	defer func() {
		for _, name := range created {
			if _, err := admin.ExecContext(context.Background(), `DROP DATABASE "`+name+`"`); err != nil {
				t.Errorf("cleanup disposable database: %v", err)
			}
		}
	}()
	urls := map[string]string{}
	for _, id := range []string{"alpha", "beta"} {
		name := prefix + "_" + id
		if _, err := admin.ExecContext(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
			t.Fatal(err)
		}
		created = append(created, name)
		value := *base
		value.Path = "/" + name
		urls[id] = value.String()
	}
	projects := []project.Project{{ID: "alpha"}, {ID: "beta"}}
	connections, err := OpenProjectDatabases(ctx, config.Config{DatabaseMode: "per-project", ProjectDatabaseURLs: urls}, projects)
	if err != nil {
		t.Fatal(err)
	}
	defer connections.Close()
	if err := connections.EachDatabase(func(db *sql.DB) error { return auth.EnsurePostgresSchema(ctx, db, "auth") }); err != nil {
		t.Fatal(err)
	}
	for _, p := range projects {
		db, err := connections.Router.Resolve(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		var exists bool
		err = db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='auth' AND table_name='sessions' AND column_name='golara_project_id')`).Scan(&exists)
		if err != nil || !exists {
			t.Fatalf("project %s auth session schema missing: %v", p.ID, err)
		}
	}
}
