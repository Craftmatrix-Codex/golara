package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type adminDeleteAuthService struct {
	fakeAuthService
	deletedID string
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
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", response.Code, response.Body.String())
	}
	if authService.deletedID != "user-1" {
		t.Fatalf("deleted id = %q, want user-1", authService.deletedID)
	}
}
