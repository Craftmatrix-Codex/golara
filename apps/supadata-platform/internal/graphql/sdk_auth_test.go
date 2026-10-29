package graphql

import (
	"net/http/httptest"
	"testing"
)

func TestSDKAPIKeyBearerUsesConfiguredKeyRole(t *testing.T) {
	h := NewHandler(HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key", ServiceRole: "service-key"}, JWTSecret: []byte("jwt-secret")})
	r := httptest.NewRequest("POST", "/graphql/v1", nil)
	r.Header.Set("apikey", "service-key")
	r.Header.Set("Authorization", "Bearer service-key")
	claims, err := h.accessClaims(r, "service_role")
	if err != nil || claims.Role != "service_role" {
		t.Fatalf("role=%s err=%v", claims.Role, err)
	}
}
