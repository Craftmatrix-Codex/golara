package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthSignupHonorsDisableSignupSetting(t *testing.T) {
	server := NewServer(ServerOptions{
		Token: "secret", Registry: &fakeRegistry{}, Auth: fakeAuthService{}, APIKeys: APIKeyConfig{Anon: "anon-key"}, AuthSettings: AuthSettings{DisableSignup: true},
	})
	request := httptest.NewRequest(http.MethodPost, "/auth/v1/signup", strings.NewReader(`{"email":"user@example.com","password":"password-123456"}`))
	request.Header.Set("apikey", "anon-key")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("disabled signup status = %d, want 403", response.Code)
	}
}
