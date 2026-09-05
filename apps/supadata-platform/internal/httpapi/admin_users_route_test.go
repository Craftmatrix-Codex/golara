package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/auth"
)

type adminAuthService struct {
	fakeAuthService
	called bool
}

func (service *adminAuthService) ListUsers(_ context.Context, page, perPage int) ([]auth.User, int, error) {
	service.called = page == 2 && perPage == 3
	return []auth.User{{ID: "user-1", Email: "user@example.com", Role: "authenticated"}}, 7, nil
}

func TestAuthAdminListUsersRequiresServiceRoleAndReturnsPagination(t *testing.T) {
	authService := &adminAuthService{}
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}, Auth: authService, APIKeys: APIKeyConfig{Anon: "anon-key", ServiceRole: "service-key"}})
	request := httptest.NewRequest(http.MethodGet, "/auth/v1/admin/users?page=2&per_page=3", nil)
	request.Header.Set("apikey", "service-key")
	request.Header.Set("Authorization", "Bearer service-key")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var body struct {
		Users []auth.User `json:"users"`
		Total int         `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !authService.called || len(body.Users) != 1 || body.Total != 7 {
		t.Fatalf("response=%+v called=%v", body, authService.called)
	}
}

func TestAuthAdminListUsersRejectsAnonKey(t *testing.T) {
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}, Auth: &adminAuthService{}, APIKeys: APIKeyConfig{Anon: "anon-key", ServiceRole: "service-key"}})
	request := httptest.NewRequest(http.MethodGet, "/auth/v1/admin/users", nil)
	request.Header.Set("apikey", "anon-key")
	request.Header.Set("Authorization", "Bearer anon-key")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}
