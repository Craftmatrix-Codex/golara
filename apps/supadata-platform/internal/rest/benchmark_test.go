package rest

import (
	"net/url"
	"testing"
)

func BenchmarkBuildSelectQuery(b *testing.B) {
	values := url.Values{"id": {"eq.42"}, "archived": {"is.false"}, "limit": {"100"}, "order": {"created_at.desc"}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := BuildSelectQuery("public", "todos", values); err != nil {
			b.Fatal(err)
		}
	}
}
