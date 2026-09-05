package rest

import "testing"

func TestBuildRPCQueryUsesNamedArguments(t *testing.T) {
	query, err := BuildRPCQuery("public", "add_todo", map[string]any{"title": "Fix auth", "priority": 1})
	if err != nil {
		t.Fatal(err)
	}
	if query.SQL != `SELECT * FROM "public"."add_todo"("priority" := $1, "title" := $2)` {
		t.Fatalf("SQL = %q", query.SQL)
	}
	if len(query.Args) != 2 || query.Args[0] != 1 || query.Args[1] != "Fix auth" {
		t.Fatalf("Args = %#v", query.Args)
	}
}
