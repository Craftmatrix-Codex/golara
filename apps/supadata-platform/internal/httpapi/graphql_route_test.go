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

func TestProjectScopedSDKPathsUseProjectScopeAndNormalizeThePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "rest", path: "/api/projects/demo/rest/v1/items", want: "/rest/v1/items"},
		{name: "graphql", path: "/api/projects/demo/graphql/v1", want: "/graphql/v1"},
		{name: "storage", path: "/api/projects/demo/storage/v1/bucket", want: "/storage/v1/bucket"},
		{name: "realtime", path: "/api/projects/demo/realtime/v1/websocket", want: "/realtime/v1/websocket"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.URL.Path != test.want {
					t.Fatalf("normalized path = %s, want %s", request.URL.Path, test.want)
				}
				scope, ok := project.ScopeFromContext(request.Context())
				if !ok || scope.ID != "demo" {
					t.Fatalf("project scope = %+v, present = %v", scope, ok)
				}
				response.WriteHeader(http.StatusOK)
			})
			server := NewServer(ServerOptions{
				ProjectResolver:     projectScopeResolverStub{projects: map[string]project.Project{"demo": {ID: "demo"}}},
				RequireProjectScope: true,
				REST:                service,
				GraphQL:             service,
				Storage:             service,
				Realtime:            service,
			})
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			request.Header.Set("apikey", "anon-key")
			response := httptest.NewRecorder()

			server.Handler().ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", response.Code)
			}
		})
	}
}
