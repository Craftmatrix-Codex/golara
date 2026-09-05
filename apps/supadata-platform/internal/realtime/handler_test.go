package realtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
)

func realtimeTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	server := httptest.NewServer(NewHandler(HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon", ServiceRole: "service"}}))
	t.Cleanup(server.Close)
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Scheme = "ws"
	parsed.Path = "/realtime/v1/websocket"
	parsed.RawQuery = "apikey=anon&vsn=1.0.0"
	return server, parsed.String()
}

func TestRealtimeRejectsMissingAPIKey(t *testing.T) {
	server := httptest.NewServer(NewHandler(HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon"}}))
	defer server.Close()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Scheme = "ws"
	parsed.Path = "/realtime/v1/websocket"
	connection, response, dialErr := websocket.DefaultDialer.Dial(parsed.String(), nil)
	if connection != nil {
		connection.Close()
	}
	if dialErr == nil || response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("dial error=%v response=%v", dialErr, response)
	}
}

func TestRealtimeJoinAndHeartbeatProtocol(t *testing.T) {
	_, websocketURL := realtimeTestServer(t)
	connection, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()

	join := []any{nil, "1", "realtime:public:public", "phx_join", map[string]any{}}
	if err := connection.WriteJSON(join); err != nil {
		t.Fatal(err)
	}
	assertReply(t, connection, "1", "realtime:public:public", "ok")

	heartbeat := []any{nil, "2", "phoenix", "heartbeat", map[string]any{}}
	if err := connection.WriteJSON(heartbeat); err != nil {
		t.Fatal(err)
	}
	assertReply(t, connection, "2", "phoenix", "ok")
}

func TestRealtimeRejectsNonPublicJoinTopic(t *testing.T) {
	_, websocketURL := realtimeTestServer(t)
	connection, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.WriteJSON([]any{nil, "1", "realtime:private:secret", "phx_join", map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	assertReply(t, connection, "1", "realtime:private:secret", "error")
}

func assertReply(t *testing.T, connection *websocket.Conn, reference, topic, status string) {
	t.Helper()
	var message []json.RawMessage
	if err := connection.ReadJSON(&message); err != nil {
		t.Fatal(err)
	}
	if len(message) != 5 {
		t.Fatalf("message length=%d message=%s", len(message), message)
	}
	var gotReference, gotTopic, event string
	if err := json.Unmarshal(message[1], &gotReference); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(message[2], &gotTopic); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(message[3], &event); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(message[4], &payload); err != nil {
		t.Fatal(err)
	}
	if gotReference != reference || gotTopic != topic || event != "phx_reply" || payload.Status != status {
		t.Fatalf("reply ref=%q topic=%q event=%q status=%q", gotReference, gotTopic, event, payload.Status)
	}
}
