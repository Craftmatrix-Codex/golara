package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthenticatedSelectionReturnsSelectedProject(t *testing.T) {
	registry := &fakeRegistry{projects: []Project{{ID: "demo", Name: "Demo", Status: "ready"}}}
	server := NewServer(ServerOptions{Token: "secret", Registry: registry})
	request := httptest.NewRequest(http.MethodPost, "/api/projects/demo/select", strings.NewReader("{}"))
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if !registry.projects[0].Current {
		t.Fatal("selection was not applied to the registry")
	}
}
