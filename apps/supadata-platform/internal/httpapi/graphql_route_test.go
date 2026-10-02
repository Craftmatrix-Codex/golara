package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

func TestProjectGraphQLPathUsesProjectScopeAndGraphQLHandler(t *testing.T) {
	graphqlHandler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/graphql/v1" {
			t.Fatalf("GraphQL path = %s", request.URL.Path)
		}
		scope, ok := project.ScopeFromContext(request.Context())
		if !ok || scope.ID != "demo" {
			t.Fatalf("GraphQL project scope = %+v, present = %v", scope, ok)
		}
		response.WriteHeader(http.StatusOK)
	})
	server := NewServer(ServerOptions{
		ProjectResolver:     projectScopeResolverStub{projects: map[string]project.Project{"demo": {ID: "demo"}}},
		RequireProjectScope: true,
		GraphQL:             graphqlHandler,
	})
	request := httptest.NewRequest(http.MethodPost, "/api/projects/demo/api/graphql", nil)
	request.Header.Set("apikey", "anon-key")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
}
