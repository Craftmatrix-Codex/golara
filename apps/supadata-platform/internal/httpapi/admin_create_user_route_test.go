package httpapi

import (
	"context"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/auth"
	"net/http/httptest"
	"strings"
	"testing"
)

type adminCreateAuthService struct {
	fakeAuthService
	confirmed bool
}

func (s *adminCreateAuthService) AdminCreateUser(_ context.Context, email, password string, metadata map[string]any, confirmed bool) (auth.User, error) {
	s.confirmed = confirmed
	return auth.User{ID: "user-1", Email: email, Role: "authenticated", UserMetadata: metadata}, nil
}

func TestSDKAdminCreatesConfirmedUserOnlyWithServiceRole(t *testing.T) {
	service := &adminCreateAuthService{}
	server := NewServer(ServerOptions{Auth: service, APIKeys: APIKeyConfig{Anon: "anon", ServiceRole: "service"}})
	for _, key := range []string{"anon", "service"} {
		r := httptest.NewRequest("POST", "/auth/v1/admin/users", strings.NewReader(`{"email":"sdk@example.invalid","password":"password","email_confirm":true,"user_metadata":{"fixture":true}}`))
		r.Header.Set("apikey", key)
		r.Header.Set("Authorization", "Bearer "+key)
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, r)
		want := 401
		if key == "service" {
			want = 200
		}
		if w.Code != want {
			t.Fatalf("key=%s status=%d body=%s", key, w.Code, w.Body.String())
		}
	}
	if !service.confirmed {
		t.Fatal("email confirmation flag was lost")
	}
}
