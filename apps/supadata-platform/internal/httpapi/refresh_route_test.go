package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/auth"
)

type refreshAuthService struct{}

func (refreshAuthService) SignUp(context.Context, string, string, map[string]any) (auth.SessionResponse, error) {
	return auth.SessionResponse{}, nil
}
func (refreshAuthService) SignIn(context.Context, string, string) (auth.SessionResponse, error) {
	return auth.SessionResponse{}, nil
}
func (refreshAuthService) Refresh(context.Context, string) (auth.SessionResponse, error) {
	return auth.SessionResponse{AccessToken: "new-access", RefreshToken: "new-refresh", TokenType: "bearer"}, nil
}

func TestPasswordRefreshRouteReturnsRotatedSession(t *testing.T) {
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}, Auth: refreshAuthService{}, APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodPost, "/auth/v1/token?grant_type=refresh_token", strings.NewReader(`{"refresh_token":"old-refresh"}`))
	request.Header.Set("apikey", "anon-key")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"refresh_token":"new-refresh"`) {
		t.Fatalf("response did not contain rotated refresh token: %s", response.Body.String())
	}
}
