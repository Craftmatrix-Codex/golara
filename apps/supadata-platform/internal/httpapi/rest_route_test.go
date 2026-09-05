package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRESTRouteDelegatesToConfiguredRESTHandler(t *testing.T) {
	called := false
	restHandler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		called = true
		if request.URL.Path != "/rest/v1/todos" {
			t.Errorf("path = %s, want /rest/v1/todos", request.URL.Path)
		}
		response.WriteHeader(http.StatusOK)
	})
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}, REST: restHandler})
	request := httptest.NewRequest(http.MethodGet, "/rest/v1/todos", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !called {
		t.Fatalf("status=%d called=%v, want delegated 200", response.Code, called)
	}
}
