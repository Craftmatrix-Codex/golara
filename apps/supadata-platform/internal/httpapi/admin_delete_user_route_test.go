package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type adminDeleteAuthService struct {
	fakeAuthService
	deletedID string
	soft      bool
}

func (service *adminDeleteAuthService) DeleteUser(_ context.Context, id string) error {
	service.deletedID = id
	return nil
}

func TestAuthAdminDeleteUserRequiresServiceRole(t *testing.T) {
	authService := &adminDeleteAuthService{}
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}, Auth: authService, APIKeys: APIKeyConfig{ServiceRole: "service-key"}})
	request := httptest.NewRequest(http.MethodDelete, "/auth/v1/admin/users/user-1", nil)
	request.Header.Set("apikey", "service-key")
	request.Header.Set("Authorization", "Bearer service-key")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if authService.deletedID != "user-1" {
		t.Fatalf("deleted id = %q, want user-1", authService.deletedID)
	}
}

func (service *adminDeleteAuthService) DeleteUserWithOptions(_ context.Context, id string, soft bool) error {
	service.deletedID, service.soft = id, soft
	return nil
}

func TestAuthAdminDeleteHonorsSoftDeleteOption(t *testing.T) {
	for _, soft := range []bool{true, false} {
		service := &adminDeleteAuthService{}
		server := NewServer(ServerOptions{Auth: service, APIKeys: APIKeyConfig{ServiceRole: "service-key"}})
		body := `{"should_soft_delete":false}`
		if soft {
			body = `{"should_soft_delete":true}`
		}
		request := httptest.NewRequest(http.MethodDelete, "/auth/v1/admin/users/user-1", strings.NewReader(body))
		request.Header.Set("apikey", "service-key")
		request.Header.Set("Authorization", "Bearer service-key")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != 200 || service.deletedID != "user-1" || service.soft != soft {
			t.Fatalf("status=%d id=%s soft=%v want=%v", response.Code, service.deletedID, service.soft, soft)
		}
	}
}

func TestAuthAdminDeleteRejectsUnauthorizedOrMalformedOptions(t *testing.T) {
	for _, tc := range []struct {
		key, body string
		status    int
	}{
		{"anon-key", `{"should_soft_delete":true}`, 401},
		{"service-key", `{"should_soft_delete":"yes"}`, 400},
		{"service-key", `{`, 400},
		{"service-key", `{} garbage`, 400},
		{"service-key", `{} {"should_soft_delete":true}`, 400},
		{"service-key", `{}` + strings.Repeat(" ", 1<<20), 400},
	} {
		service := &adminDeleteAuthService{}
		server := NewServer(ServerOptions{Auth: service, APIKeys: APIKeyConfig{Anon: "anon-key", ServiceRole: "service-key"}})
		request := httptest.NewRequest(http.MethodDelete, "/auth/v1/admin/users/user-1", strings.NewReader(tc.body))
		request.Header.Set("apikey", tc.key)
		request.Header.Set("Authorization", "Bearer "+tc.key)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != tc.status || service.deletedID != "" {
			t.Fatalf("status=%d want=%d deleted=%s", response.Code, tc.status, service.deletedID)
		}
	}
}
