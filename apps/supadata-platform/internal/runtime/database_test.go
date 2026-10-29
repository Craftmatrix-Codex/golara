package runtime

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/config"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

func TestOpenProjectDatabasesRejectsIncompletePerProjectConfiguration(t *testing.T) {
	cfg := config.Config{
		DatabaseMode:        "per-project",
		ProjectDatabaseURLs: map[string]string{"alpha": "postgres://alpha"},
	}
	_, err := OpenProjectDatabases(context.Background(), cfg, []project.Project{{ID: "alpha"}, {ID: "beta"}})
	if err == nil {
		t.Fatal("expected missing beta database URL error")
	}
}

func TestOpenProjectDatabasesReturnsNilWhenNoDatabaseIsConfigured(t *testing.T) {
	connections, err := OpenProjectDatabases(context.Background(), config.Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if connections != nil {
		t.Fatal("expected no database connections")
	}
}

func TestEachDatabaseVisitsEveryUniqueConnectionAndPropagatesErrors(t *testing.T) {
	first, second := &sql.DB{}, &sql.DB{}
	connections := &DatabaseConnections{all: []*sql.DB{first, second}}
	var visited []*sql.DB
	expected := errors.New("schema reconciliation failed")
	err := connections.EachDatabase(func(db *sql.DB) error {
		visited = append(visited, db)
		if db == second {
			return expected
		}
		return nil
	})
	if !errors.Is(err, expected) || len(visited) != 2 || visited[0] != first || visited[1] != second {
		t.Fatalf("visited=%v error=%v", visited, err)
	}
}
