package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/auth"
)

type userAuthService struct{ fakeAuthService }

func (userAuthService) GetUserByAccessToken(context.Context, string) (auth.User, error) {
	return auth.User{ID: "user-id", Email: "user@example.com", Role: "authenticated"}, nil
}

func TestAuthUserRouteRequiresBearerAndReturnsPersistedUser(t *testing.T) {
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}, Auth: userAuthService{}, APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodGet, "/auth/v1/user", nil)
	request.Header.Set("apikey", "anon-key")
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("user status = %d, want 200", response.Code)
	}
	if !strings.Contains(response.Body.String(), "user@example.com") {
		t.Fatalf("user response = %q, want persisted email", response.Body.String())
	}
}

func TestAuthUserRouteRejectsMissingBearer(t *testing.T) {
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}, Auth: userAuthService{}, APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodGet, "/auth/v1/user", nil)
	request.Header.Set("apikey", "anon-key")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer status = %d, want 401", response.Code)
	}
}
