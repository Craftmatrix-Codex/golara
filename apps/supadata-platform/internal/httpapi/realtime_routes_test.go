package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerRoutesRealtimeRequestsToRealtimeHandler(t *testing.T) {
	realtime := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Realtime-Test", "routed")
		response.WriteHeader(http.StatusNoContent)
	})
	server := NewServer(ServerOptions{Realtime: realtime})
	request := httptest.NewRequest(http.MethodGet, "/realtime/v1/websocket", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || response.Header().Get("X-Realtime-Test") != "routed" {
		t.Fatalf("status=%d headers=%v", response.Code, response.Header())
	}
}

func TestServerReturns503WhenRealtimeIsNotConfigured(t *testing.T) {
	server := NewServer(ServerOptions{})
	request := httptest.NewRequest(http.MethodGet, "/realtime/v1/websocket", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
