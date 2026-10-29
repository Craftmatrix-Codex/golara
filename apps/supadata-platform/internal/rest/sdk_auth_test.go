package rest

import (
	"net/http/httptest"
	"testing"
)

func TestSDKAPIKeyBearerUsesConfiguredKeyRole(t *testing.T) {
	h := NewHandler(nil, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key", ServiceRole: "service-key"}, JWTSecret: []byte("jwt-secret")})
	for _, tc := range []struct{ key, role string }{{"anon-key", "anon"}, {"service-key", "service_role"}} {
		r := httptest.NewRequest("GET", "/rest/v1/todos", nil)
		r.Header.Set("apikey", tc.key)
		r.Header.Set("Authorization", "Bearer "+tc.key)
		claims, err := h.accessClaims(r)
		if err != nil || claims.Role != tc.role {
			t.Fatalf("role=%s err=%v, want %s", claims.Role, err, tc.role)
		}
	}
	r := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	r.Header.Set("apikey", "anon-key")
	r.Header.Set("Authorization", "Bearer service-key")
	if _, err := h.accessClaims(r); err == nil {
		t.Fatal("anon key must not gain service-role access from a different bearer")
	}
}
