package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

func TestStorageRejectsCrossProjectBucket(t *testing.T) {
	store := newMemoryStore()
	if _, err := store.Put(context.Background(), "supadata-alpha", "private.txt", "text/plain", strings.NewReader("alpha"), -1); err != nil {
		t.Fatalf("seed alpha object: %v", err)
	}
	if _, err := store.Put(context.Background(), "supadata-beta", "private.txt", "text/plain", strings.NewReader("beta"), -1); err != nil {
		t.Fatalf("seed beta object: %v", err)
	}
	handler := NewHandler(HandlerOptions{Store: store, APIKeys: APIKeyConfig{ServiceRole: "service"}})

	alphaRequest := httptest.NewRequest(http.MethodGet, "/storage/v1/object/supadata-alpha/private.txt", nil)
	alphaRequest.Header.Set("apikey", "service")
	alphaRequest = alphaRequest.WithContext(project.WithScope(alphaRequest.Context(), project.Project{
		ID:    "alpha",
		Scope: project.ResourceScope{Storage: project.StorageScope{Bucket: "supadata-alpha"}},
	}))
	alphaResponse := httptest.NewRecorder()
	handler.ServeHTTP(alphaResponse, alphaRequest)
	if alphaResponse.Code != http.StatusOK || alphaResponse.Body.String() != "alpha" {
		t.Fatalf("alpha read = %d %q, want 200 alpha", alphaResponse.Code, alphaResponse.Body.String())
	}

	betaRequest := httptest.NewRequest(http.MethodGet, "/storage/v1/object/supadata-beta/private.txt", nil)
	betaRequest.Header.Set("apikey", "service")
	betaRequest = betaRequest.WithContext(project.WithScope(betaRequest.Context(), project.Project{
		ID:    "beta",
		Scope: project.ResourceScope{Storage: project.StorageScope{Bucket: "supadata-beta"}},
	}))
	betaResponse := httptest.NewRecorder()
	handler.ServeHTTP(betaResponse, betaRequest)
	if betaResponse.Code != http.StatusOK || betaResponse.Body.String() != "beta" {
		t.Fatalf("beta read = %d %q, want 200 beta", betaResponse.Code, betaResponse.Body.String())
	}

	crossRequest := httptest.NewRequest(http.MethodGet, "/storage/v1/object/supadata-beta/private.txt", nil)
	crossRequest.Header.Set("apikey", "service")
	crossRequest = crossRequest.WithContext(project.WithScope(crossRequest.Context(), project.Project{
		ID:    "alpha",
		Scope: project.ResourceScope{Storage: project.StorageScope{Bucket: "supadata-alpha"}},
	}))
	crossResponse := httptest.NewRecorder()
	handler.ServeHTTP(crossResponse, crossRequest)
	if crossResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-project read = %d, want 404", crossResponse.Code)
	}
}
