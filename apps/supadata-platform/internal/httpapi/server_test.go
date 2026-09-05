package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeRegistry struct {
	projects []Project
	current  *Project
}

func (r *fakeRegistry) ListProjects(context.Context) ([]Project, error)  { return r.projects, nil }
func (r *fakeRegistry) CurrentProject(context.Context) (*Project, error) { return r.current, nil }
func (r *fakeRegistry) CreateProject(_ context.Context, name, id string) (Project, error) {
	project := Project{ID: id, Name: name, Status: "registered", Current: len(r.projects) == 0}
	r.projects = append(r.projects, project)
	if r.current == nil {
		r.current = &r.projects[0]
	}
	return project, nil
}
func (r *fakeRegistry) SelectProject(_ context.Context, id string) (Project, error) {
	for index := range r.projects {
		if r.projects[index].ID == id {
			r.projects[index].Current = true
			r.current = &r.projects[index]
			return r.projects[index], nil
		}
	}
	return Project{}, nil
}

func TestHealthIsPublicAndProjectManagementRequiresBearer(t *testing.T) {
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}})

	health := httptest.NewRecorder()
	server.Handler().ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", health.Code)
	}

	protected := httptest.NewRecorder()
	server.Handler().ServeHTTP(protected, httptest.NewRequest(http.MethodGet, "/api/projects", nil))
	if protected.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", protected.Code)
	}
}

func TestAuthenticatedProjectListPreservesStudioResponseShape(t *testing.T) {
	registry := &fakeRegistry{projects: []Project{{ID: "demo", Name: "Demo", Status: "ready", Current: true}}}
	server := NewServer(ServerOptions{Token: "secret", Registry: registry})
	request := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}

	var payload struct {
		Projects []Project `json:"projects"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Projects) != 1 || payload.Projects[0].ID != "demo" {
		t.Fatalf("unexpected projects payload: %+v", payload.Projects)
	}
}

func TestCreateProjectReturns201AndJSONProject(t *testing.T) {
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}})
	request := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"name":"Demo Project"}`))
	request.Header.Set("Authorization", "Bearer secret")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", response.Code)
	}
}
