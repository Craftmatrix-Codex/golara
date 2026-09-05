package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerRoutesStorageRequestsToStorageHandler(t *testing.T) {
	storage := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Storage-Test", "routed")
		response.WriteHeader(http.StatusNoContent)
	})
	server := NewServer(ServerOptions{Storage: storage})
	request := httptest.NewRequest(http.MethodGet, "/storage/v1/object/public/avatars/file.txt", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || response.Header().Get("X-Storage-Test") != "routed" {
		t.Fatalf("status=%d headers=%v", response.Code, response.Header())
	}
}

func TestServerReturns503WhenStorageIsNotConfigured(t *testing.T) {
	server := NewServer(ServerOptions{})
	request := httptest.NewRequest(http.MethodGet, "/storage/v1/object/public/avatars/file.txt", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
