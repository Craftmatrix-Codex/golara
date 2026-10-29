package storage

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

func TestStorageReadPrivacy(t *testing.T) {
	for _, tc := range []struct {
		name, path, key string
		public          bool
		want            int
	}{
		{"anonymous private download", "/storage/v1/object/avatars/file.txt", "anon", false, 401},
		{"private bucket public URL", "/storage/v1/object/public/avatars/file.txt", "anon", false, 404},
		{"public bucket direct URL without key", "/storage/v1/object/public/avatars/file.txt", "", true, 200},
		{"private service download", "/storage/v1/object/avatars/file.txt", "service", false, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newMemoryStore()
			_, err := store.Put(context.Background(), "avatars", "file.txt", "text/plain", strings.NewReader("private"), 7)
			if err != nil {
				t.Fatal(err)
			}
			metadata, _ := json.Marshal(map[string]any{"id": "avatars", "public": tc.public})
			h := NewHandler(HandlerOptions{Store: store, APIKeys: APIKeyConfig{Anon: "anon", ServiceRole: "service"}})
			r := httptest.NewRequest("GET", tc.path, nil)
			r.Header.Set("apikey", tc.key)
			r = r.WithContext(project.WithScope(r.Context(), project.Project{ID: "default", Scope: project.ResourceScope{Storage: project.StorageScope{Bucket: "avatars", Buckets: []json.RawMessage{metadata}}}}))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestStorageAnonymousCannotListPrivateObjects(t *testing.T) {
	h := NewHandler(HandlerOptions{Store: newMemoryStore(), APIKeys: APIKeyConfig{Anon: "anon"}})
	r := httptest.NewRequest("POST", "/storage/v1/object/list/avatars", strings.NewReader(`{"prefix":""}`))
	r.Header.Set("apikey", "anon")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("status=%d want=401", w.Code)
	}
}
