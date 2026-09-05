package rest

import (
	"net/url"
	"testing"
)

func TestBuildSelectQueryUsesParameterizedFiltersAndSafeIdentifiers(t *testing.T) {
	query, err := BuildSelectQuery("public", "todos", url.Values{
		"select": {"id,name"},
		"id":     {"eq.42"},
		"title":  {"ilike.%bug%"},
		"order":  {"created_at.desc"},
		"limit":  {"10"},
		"offset": {"20"},
	})
	if err != nil {
		t.Fatalf("BuildSelectQuery() error = %v", err)
	}
	wantSQL := `SELECT "id", "name" FROM "public"."todos" WHERE "id" = $1 AND "title" ILIKE $2 ORDER BY "created_at" DESC LIMIT 10 OFFSET 20`
	if query.SQL != wantSQL {
		t.Fatalf("SQL = %q, want %q", query.SQL, wantSQL)
	}
	if len(query.Args) != 2 || query.Args[0] != "42" || query.Args[1] != "%bug%" {
		t.Fatalf("Args = %#v, want parameter values", query.Args)
	}
}

func TestBuildSelectQueryRejectsInjectionInIdentifiers(t *testing.T) {
	if _, err := BuildSelectQuery("public", "todos;drop table users", url.Values{}); err == nil {
		t.Fatal("unsafe table identifier was accepted")
	}
	if _, err := BuildSelectQuery("public", "todos", url.Values{"select": {"id,pg_sleep(1)"}}); err == nil {
		t.Fatal("unsafe select identifier was accepted")
	}
}

func TestBuildSelectQuerySupportsPostgRESTNullAndBooleanFilters(t *testing.T) {
	query, err := BuildSelectQuery("public", "todos", url.Values{
		"deleted_at": {"is.null"},
		"archived":   {"is.false"},
	})
	if err != nil {
		t.Fatalf("BuildSelectQuery() error = %v", err)
	}
	wantSQL := `SELECT * FROM "public"."todos" WHERE "archived" IS FALSE AND "deleted_at" IS NULL`
	if query.SQL != wantSQL {
		t.Fatalf("SQL = %q, want %q", query.SQL, wantSQL)
	}
	if len(query.Args) != 0 {
		t.Fatalf("Args = %#v, want none", query.Args)
	}
}

func TestBuildSelectQuerySupportsInAndNotFilters(t *testing.T) {
	query, err := BuildSelectQuery("public", "todos", url.Values{
		"id":    {"in.(1,2,3)"},
		"title": {"not.ilike.%draft%"},
	})
	if err != nil {
		t.Fatalf("BuildSelectQuery() error = %v", err)
	}
	wantSQL := `SELECT * FROM "public"."todos" WHERE "id" IN ($1, $2, $3) AND NOT ("title" ILIKE $4)`
	if query.SQL != wantSQL {
		t.Fatalf("SQL = %q, want %q", query.SQL, wantSQL)
	}
	if len(query.Args) != 4 || query.Args[0] != "1" || query.Args[1] != "2" || query.Args[2] != "3" || query.Args[3] != "%draft%" {
		t.Fatalf("Args = %#v, want four values", query.Args)
	}
}

func TestBuildSelectQuerySupportsOrFilters(t *testing.T) {
	query, err := BuildSelectQuery("public", "todos", url.Values{"or": {"(id.eq.1,title.ilike.%bug%)"}})
	if err != nil {
		t.Fatalf("BuildSelectQuery() error = %v", err)
	}
	wantSQL := `SELECT * FROM "public"."todos" WHERE ("id" = $1 OR "title" ILIKE $2)`
	if query.SQL != wantSQL || len(query.Args) != 2 || query.Args[0] != "1" || query.Args[1] != "%bug%" {
		t.Fatalf("SQL = %q args=%#v", query.SQL, query.Args)
	}
}

func FuzzBuildSelectQueryNeverPanics(f *testing.F) {
	f.Add("todos")
	f.Add(`todos; DROP TABLE users`)
	f.Add(`todos" FROM users --`)
	f.Fuzz(func(t *testing.T, table string) {
		query, err := BuildSelectQuery("public", table, nil)
		if err == nil && query.SQL == "" {
			t.Fatal("successful query was empty")
		}
	})
}
