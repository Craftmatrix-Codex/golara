package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerRoutesFunctionRequestsToFunctionHandler(t *testing.T) {
	functionHandler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Function-Test", "routed")
		response.WriteHeader(http.StatusAccepted)
	})
	server := NewServer(ServerOptions{Functions: functionHandler})
	request := httptest.NewRequest(http.MethodPost, "/functions/v1/hello", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusAccepted || response.Header().Get("X-Function-Test") != "routed" {
		t.Fatalf("status=%d headers=%v", response.Code, response.Header())
	}
}

func TestServerReturns503WhenFunctionsAreNotConfigured(t *testing.T) {
	server := NewServer(ServerOptions{})
	request := httptest.NewRequest(http.MethodPost, "/functions/v1/hello", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestProjectScopedSDKPathNormalizesFunctions(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/projects/demo/functions/v1/hello", nil)
	normalized := normalizeProjectScopedSDKPath(request)
	if normalized.URL.Path != "/functions/v1/hello" {
		t.Fatalf("unexpected path: %s", normalized.URL.Path)
	}
	if normalized.Header.Get("X-Supadata-Project") != "demo" {
		t.Fatalf("project scope was not attached")
	}
}
