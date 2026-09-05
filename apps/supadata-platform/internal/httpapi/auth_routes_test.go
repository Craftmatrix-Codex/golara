package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/auth"
)

type fakeAuthService struct{}

func (fakeAuthService) SignUp(context.Context, string, string, map[string]any) (auth.SessionResponse, error) {
	return auth.SessionResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: "bearer", User: auth.User{ID: "user-1"}}, nil
}
func (fakeAuthService) SignIn(context.Context, string, string) (auth.SessionResponse, error) {
	return auth.SessionResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: "bearer", User: auth.User{ID: "user-1"}}, nil
}
func (fakeAuthService) Refresh(context.Context, string) (auth.SessionResponse, error) {
	return auth.SessionResponse{AccessToken: "new-access", RefreshToken: "new-refresh", TokenType: "bearer"}, nil
}

func TestAuthHealthMatchesGoTrueResponseShape(t *testing.T) {
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}})
	request := httptest.NewRequest(http.MethodGet, "/auth/v1/health", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if got := response.Body.String(); !strings.Contains(got, `"name":"Auth"`) || !strings.Contains(got, `"description":"Auth is a user registration and authentication API"`) {
		t.Fatalf("unexpected health response: %s", got)
	}
}

func TestAuthSignupAndPasswordTokenRoutesUseAuthService(t *testing.T) {
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}, Auth: fakeAuthService{}, APIKeys: APIKeyConfig{Anon: "anon-key"}})

	signup := httptest.NewRecorder()
	signupRequest := httptest.NewRequest(http.MethodPost, "/auth/v1/signup", strings.NewReader(`{"email":"user@example.com","password":"password-123456","data":{"name":"User"}}`))
	signupRequest.Header.Set("Content-Type", "application/json")
	signupRequest.Header.Set("apikey", "anon-key")
	server.Handler().ServeHTTP(signup, signupRequest)
	if signup.Code != http.StatusOK {
		t.Fatalf("signup status = %d, want 200", signup.Code)
	}
	if !strings.Contains(signup.Body.String(), `"access_token":"access"`) {
		t.Fatalf("signup response did not contain session payload: %s", signup.Body.String())
	}

	signin := httptest.NewRecorder()
	signinRequest := httptest.NewRequest(http.MethodPost, "/auth/v1/token?grant_type=password", strings.NewReader(`{"email":"user@example.com","password":"password-123456"}`))
	signinRequest.Header.Set("Content-Type", "application/json")
	signinRequest.Header.Set("apikey", "anon-key")
	server.Handler().ServeHTTP(signin, signinRequest)
	if signin.Code != http.StatusOK {
		t.Fatalf("signin status = %d, want 200", signin.Code)
	}
}

func TestAuthRoutesReturn503WhenAuthStorageIsNotConfigured(t *testing.T) {
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}, APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodPost, "/auth/v1/signup", strings.NewReader(`{"email":"user@example.com","password":"password-123456"}`))
	request.Header.Set("apikey", "anon-key")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
}
