package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGETTableRejectsInvalidAccessToken(t *testing.T) {
	handler := NewHandler(nil, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}, JWTSecret: []byte("jwt-secret")})
	request := httptest.NewRequest(http.MethodGet, "/rest/v1/todos", nil)
	request.Header.Set("apikey", "anon-key")
	request.Header.Set("Authorization", "Bearer forged-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}
