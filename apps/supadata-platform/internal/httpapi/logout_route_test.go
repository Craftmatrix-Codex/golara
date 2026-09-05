package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/auth"
)

type logoutAuthService struct {
	fakeAuthService
	accessToken string
}

func (service *logoutAuthService) Logout(_ context.Context, accessToken string) error {
	service.accessToken = accessToken
	return nil
}

func TestAuthLogoutRevokesBearerSession(t *testing.T) {
	authService := &logoutAuthService{}
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}, Auth: authService, APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodPost, "/auth/v1/logout", nil)
	request.Header.Set("apikey", "anon-key")
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if authService.accessToken != "access-token" {
		t.Fatalf("logout token = %q, want access-token", authService.accessToken)
	}
}

func TestAuthLogoutRequiresBearerToken(t *testing.T) {
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}, Auth: &logoutAuthService{}, APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodPost, "/auth/v1/logout", nil)
	request.Header.Set("apikey", "anon-key")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}

var _ AuthService = (*logoutAuthService)(nil)
var _ auth.User = auth.User{}
