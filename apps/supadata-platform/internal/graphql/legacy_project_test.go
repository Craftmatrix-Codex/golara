package graphql

import (
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLegacyUserTokenIsRestrictedToDefaultProject(t *testing.T) {
	secret := []byte("legacy-project-test-secret")
	handler := NewHandler(HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}, JWTSecret: secret})
	for _, tc := range []struct {
		tokenProject, target string
		allowed              bool
	}{
		{"", "default", true}, {"", "alpha", false}, {"alpha", "alpha", true},
		{"alpha", "beta", false}, {"default", "alpha", false},
	} {
		t.Run(tc.tokenProject+"/"+tc.target, func(t *testing.T) {
			token, err := jwt.SignHS256(jwt.Claims{Subject: "user-id", Role: "authenticated", ProjectID: tc.tokenProject, ExpiresAt: time.Now().Add(time.Hour).Unix()}, secret)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest("GET", "/", nil)
			request.Header.Set("apikey", "anon-key")
			request.Header.Set("Authorization", "Bearer "+token)
			request = request.WithContext(project.WithScope(request.Context(), project.Project{ID: tc.target}))
			_, err = handler.accessClaims(request, "anon")
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v error=%v", tc.allowed, err)
			}
		})
	}
}

func TestConfiguredAPIKeysWorkOnNondefaultProject(t *testing.T) {
	secret := []byte("legacy-project-test-secret")
	for _, signed := range []bool{false, true} {
		anon, service := "anon-key", "service-key"
		if signed {
			var err error
			anon, err = jwt.SignHS256(jwt.Claims{Role: "anon", Issuer: "legacy-key-issuer"}, secret)
			if err != nil {
				t.Fatal(err)
			}
			service, err = jwt.SignHS256(jwt.Claims{Role: "service_role", Issuer: "legacy-key-issuer"}, secret)
			if err != nil {
				t.Fatal(err)
			}
		}
		handler := NewHandler(HandlerOptions{APIKeys: APIKeyConfig{Anon: anon, ServiceRole: service}, JWTSecret: secret, Issuer: "user-issuer"})
		for _, tc := range []struct{ key, role string }{{anon, "anon"}, {service, "service_role"}} {
			for _, bearer := range []bool{false, true} {
				request := httptest.NewRequest("GET", "/", nil)
				request.Header.Set("apikey", tc.key)
				if bearer {
					request.Header.Set("Authorization", "Bearer "+tc.key)
				}
				request = request.WithContext(project.WithScope(request.Context(), project.Project{ID: "alpha"}))
				claims, err := handler.accessClaims(request, tc.role)
				if err != nil || claims.Role != tc.role {
					t.Fatalf("signed=%v bearer=%v role=%s want=%s error=%v", signed, bearer, claims.Role, tc.role, err)
				}
			}
		}
	}
}
