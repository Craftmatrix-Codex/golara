package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthUserRoutesRequireConfiguredAPIKey(t *testing.T) {
	server := NewServer(ServerOptions{
		Token:    "secret",
		Registry: &fakeRegistry{},
		Auth:     fakeAuthService{},
		APIKeys:  APIKeyConfig{Anon: "anon-key"},
	})
	request := httptest.NewRequest(http.MethodPost, "/auth/v1/signup", strings.NewReader(`{"email":"user@example.com","password":"password-123456"}`))
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status without apikey = %d, want 401", response.Code)
	}

	request.Header.Set("apikey", "anon-key")
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status with apikey = %d, want 200", response.Code)
	}
}
