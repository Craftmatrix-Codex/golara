package rest

import (
	"net/url"
	"strings"
	"testing"
)

func TestBuildUpsertQueryUsesValidatedConflictColumns(t *testing.T) {
	query, err := BuildUpsertQuery("public", "profiles", []map[string]any{{"id": "u1", "name": "Ada"}}, "id")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query.SQL, `ON CONFLICT ("id") DO UPDATE SET "name" = EXCLUDED."name" RETURNING *`) {
		t.Fatalf("query = %q", query.SQL)
	}
	if len(query.Args) != 2 || query.Args[0] != "u1" || query.Args[1] != "Ada" {
		t.Fatalf("args = %#v", query.Args)
	}
}

func TestBuildUpsertQueryRejectsMissingOrUnsafeConflictColumns(t *testing.T) {
	rows := []map[string]any{{"id": "u1"}}
	for _, conflict := range []string{"", "id);drop table users;--", "missing"} {
		if _, err := BuildUpsertQuery("public", "profiles", rows, conflict); err == nil {
			t.Fatalf("conflict %q was accepted", conflict)
		}
	}
}

func TestUpsertRouteSelectsUpsertBuilder(t *testing.T) {
	values := url.Values{"on_conflict": {"id"}}
	query, err := BuildUpsertQuery("public", "profiles", []map[string]any{{"id": "u1"}}, values.Get("on_conflict"))
	if err != nil || !strings.Contains(query.SQL, "ON CONFLICT") {
		t.Fatalf("query=%q err=%v", query.SQL, err)
	}
}
