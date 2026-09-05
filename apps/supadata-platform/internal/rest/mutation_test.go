package rest

import (
	"net/url"
	"testing"
)

func TestBuildUpdateQueryRequiresFilterAndReturnsRows(t *testing.T) {
	query, err := BuildUpdateQuery("public", "todos", map[string]any{"title": "Fixed"}, url.Values{"id": {"eq.42"}})
	if err != nil {
		t.Fatalf("BuildUpdateQuery() error = %v", err)
	}
	wantSQL := `UPDATE "public"."todos" SET "title" = $1 WHERE "id" = $2 RETURNING *`
	if query.SQL != wantSQL || len(query.Args) != 2 || query.Args[0] != "Fixed" || query.Args[1] != "42" {
		t.Fatalf("query = %#v, want filtered update", query)
	}
	if _, err := BuildUpdateQuery("public", "todos", map[string]any{"title": "unsafe"}, nil); err == nil {
		t.Fatal("unfiltered update was accepted")
	}
}

func TestBuildDeleteQueryRequiresFilter(t *testing.T) {
	query, err := BuildDeleteQuery("public", "todos", url.Values{"id": {"eq.42"}})
	if err != nil {
		t.Fatalf("BuildDeleteQuery() error = %v", err)
	}
	if query.SQL != `DELETE FROM "public"."todos" WHERE "id" = $1 RETURNING *` || len(query.Args) != 1 || query.Args[0] != "42" {
		t.Fatalf("query = %#v, want filtered delete", query)
	}
}
