package rest

import (
	"testing"
)

func TestBuildInsertQueryUsesSafeColumnsAndReturning(t *testing.T) {
	query, err := BuildInsertQuery("public", "todos", []map[string]any{{"title": "Fix auth", "id": 42}})
	if err != nil {
		t.Fatalf("BuildInsertQuery() error = %v", err)
	}
	wantSQL := `INSERT INTO "public"."todos" ("id", "title") VALUES ($1, $2) RETURNING *`
	if query.SQL != wantSQL {
		t.Fatalf("SQL = %q, want %q", query.SQL, wantSQL)
	}
	if len(query.Args) != 2 || query.Args[0] != 42 || query.Args[1] != "Fix auth" {
		t.Fatalf("Args = %#v, want sorted values", query.Args)
	}
}

func TestBuildInsertQueryRejectsEmptyOrUnsafeRows(t *testing.T) {
	if _, err := BuildInsertQuery("public", "todos", nil); err == nil {
		t.Fatal("empty rows were accepted")
	}
	if _, err := BuildInsertQuery("public", "todos", []map[string]any{{"title);drop": "x"}}); err == nil {
		t.Fatal("unsafe insert column was accepted")
	}
}
