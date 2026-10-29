package storage

import (
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStorageRejectsSignedAnonymousBearerForWrites(t *testing.T) {
	token, err := jwt.SignHS256(jwt.Claims{Role: "anon", Audience: "authenticated", ExpiresAt: time.Now().Add(time.Hour).Unix()}, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(HandlerOptions{Store: newMemoryStore(), APIKeys: APIKeyConfig{Anon: token}, JWTSecret: []byte("secret"), Audience: "authenticated"})
	r := httptest.NewRequest("POST", "/storage/v1/object/public/avatars/file.txt", strings.NewReader("file"))
	r.Header.Set("apikey", token)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("status=%d, want 401", w.Code)
	}
}

func TestStorageBatchRemovalRejectsAnotherProjectsUserToken(t *testing.T) {
	token, err := jwt.SignHS256(jwt.Claims{Subject: "user-1", Role: "authenticated", ProjectID: "alpha", Audience: "authenticated", ExpiresAt: time.Now().Add(time.Hour).Unix()}, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(HandlerOptions{Store: newMemoryStore(), APIKeys: APIKeyConfig{Anon: "anon"}, JWTSecret: []byte("secret"), Audience: "authenticated"})
	r := httptest.NewRequest("DELETE", "/storage/v1/object/avatars", strings.NewReader(`{"prefixes":["file.txt"]}`))
	r.Header.Set("apikey", "anon")
	r.Header.Set("Authorization", "Bearer "+token)
	r = r.WithContext(project.WithScope(r.Context(), project.Project{ID: "beta", Scope: project.ResourceScope{Storage: project.StorageScope{Bucket: "avatars"}}}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("status=%d, want 401", w.Code)
	}
}
