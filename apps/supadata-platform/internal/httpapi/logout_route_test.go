package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/auth"
)

type logoutAuthService struct {
	fakeAuthService
	accessToken string
	scope       string
	err         error
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

func (service *logoutAuthService) LogoutWithScope(_ context.Context, token, scope string) error {
	service.accessToken, service.scope = token, scope
	return service.err
}

func TestAuthLogoutRoutesScopesAndDoesNotHideLegacyError(t *testing.T) {
	for _, tc := range []struct {
		query, want string
		err         error
		status      int
	}{
		{"", "global", nil, 204}, {"?scope=local", "local", nil, 204},
		{"?scope=others", "others", nil, 204}, {"?scope=invalid", "", nil, 400},
		{"?scope=global", "global", fmt.Errorf("wrapped: %w", auth.ErrUnscopedLegacySessions), 409},
	} {
		service := &logoutAuthService{err: tc.err}
		server := NewServer(ServerOptions{Auth: service, APIKeys: APIKeyConfig{Anon: "anon-key"}})
		request := httptest.NewRequest(http.MethodPost, "/auth/v1/logout"+tc.query, nil)
		request.Header.Set("apikey", "anon-key")
		request.Header.Set("Authorization", "Bearer token")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != tc.status || service.scope != tc.want {
			t.Fatalf("query=%s status=%d scope=%s want=%d/%s", tc.query, response.Code, service.scope, tc.status, tc.want)
		}
	}
}

type legacyLogoutAuthService struct {
	fakeAuthService
	called bool
}

func (s *legacyLogoutAuthService) Logout(_ context.Context, _ string) error {
	s.called = true
	return nil
}
func TestLegacyLogoutServiceCannotClaimGlobalOrOthers(t *testing.T) {
	for _, tc := range []struct {
		scope  string
		status int
		called bool
	}{
		{"global", 501, false}, {"others", 501, false}, {"local", 204, true},
	} {
		service := &legacyLogoutAuthService{}
		server := NewServer(ServerOptions{Auth: service, APIKeys: APIKeyConfig{Anon: "anon-key"}})
		request := httptest.NewRequest(http.MethodPost, "/auth/v1/logout?scope="+tc.scope, nil)
		request.Header.Set("apikey", "anon-key")
		request.Header.Set("Authorization", "Bearer token")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != tc.status || service.called != tc.called {
			t.Fatalf("scope=%s status=%d called=%v", tc.scope, response.Code, service.called)
		}
	}
}
