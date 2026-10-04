package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type fakeChangeSource struct {
	subscription ChangeSubscription
	emit         func(ChangeEvent)
}

func (s *fakeChangeSource) Subscribe(_ context.Context, _ string, subscription ChangeSubscription, emit func(ChangeEvent)) (func(), error) {
	s.subscription = subscription
	s.emit = emit
	return func() {}, nil
}

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

func TestRealtimeRejectsInvalidAccessToken(t *testing.T) {
	server := httptest.NewServer(NewHandler(HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon"}, JWTSecret: []byte("secret")}))
	defer server.Close()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Scheme = "ws"
	parsed.Path = "/realtime/v1/websocket"
	parsed.RawQuery = "apikey=anon&access_token=not-a-jwt&vsn=1.0.0"
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

func TestRealtimeRejectsAnonPostgresChanges(t *testing.T) {
	source := &fakeChangeSource{}
	server := httptest.NewServer(NewHandler(HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon"}, ChangeSource: source}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	parsed.Scheme = "ws"
	parsed.Path = "/realtime/v1/websocket"
	parsed.RawQuery = "apikey=anon&vsn=1.0.0"
	connection, _, err := websocket.DefaultDialer.Dial(parsed.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	join := []any{"1", "1", "realtime:public:messages", "phx_join", map[string]any{"config": map[string]any{"postgres_changes": []any{map[string]any{"event": "*", "schema": "public", "table": "messages"}}}}}
	if err := connection.WriteJSON(join); err != nil {
		t.Fatal(err)
	}
	assertReply(t, connection, "1", "realtime:public:messages", "error")
	if source.emit != nil {
		t.Fatal("anon subscription must not reach the database change source")
	}
}

func TestRealtimeDeliversPostgresChangesFromSubscribedSource(t *testing.T) {
	source := &fakeChangeSource{}
	server := httptest.NewServer(NewHandler(HandlerOptions{APIKeys: APIKeyConfig{ServiceRole: "service"}, ChangeSource: source}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	parsed.Scheme = "ws"
	parsed.Path = "/realtime/v1/websocket"
	parsed.RawQuery = "apikey=service&vsn=1.0.0"
	connection, _, err := websocket.DefaultDialer.Dial(parsed.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	join := []any{"1", "1", "realtime:public:messages", "phx_join", map[string]any{"config": map[string]any{"postgres_changes": []any{map[string]any{"event": "INSERT", "schema": "public", "table": "messages"}}}}}
	if err := connection.WriteJSON(join); err != nil {
		t.Fatal(err)
	}
	var reply []json.RawMessage
	if err := connection.ReadJSON(&reply); err != nil {
		t.Fatal(err)
	}
	if source.subscription.Schema != "public" || source.subscription.Table != "messages" || source.subscription.Event != "INSERT" {
		t.Fatalf("unexpected subscription: %#v", source.subscription)
	}
	source.emit(ChangeEvent{Schema: "public", Table: "messages", Event: "INSERT", Record: map[string]any{"id": float64(7)}})
	var message []json.RawMessage
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	if err := connection.ReadJSON(&message); err != nil {
		t.Fatal(err)
	}
	var event string
	_ = json.Unmarshal(message[3], &event)
	if event != "postgres_changes" {
		t.Fatalf("expected postgres_changes event, got %q", event)
	}
	var payload struct {
		Data ChangeEvent `json:"data"`
	}
	if err := json.Unmarshal(message[4], &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Record["id"] != float64(7) {
		t.Fatalf("unexpected change payload: %#v", payload.Data)
	}
}

func TestRealtimeBroadcastsToJoinedClients(t *testing.T) {
	_, websocketURL := realtimeTestServer(t)
	first, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	topic := "realtime:public:room"
	for index, connection := range []*websocket.Conn{first, second} {
		reference := string(rune('1' + index))
		if err := connection.WriteJSON([]any{nil, reference, topic, "phx_join", map[string]any{}}); err != nil {
			t.Fatal(err)
		}
		assertReply(t, connection, reference, topic, "ok")
	}

	payload := map[string]any{"event": "message", "payload": map[string]any{"text": "hello"}}
	if err := first.WriteJSON([]any{nil, "3", topic, "broadcast", payload}); err != nil {
		t.Fatal(err)
	}
	var message []json.RawMessage
	_ = second.SetReadDeadline(time.Now().Add(time.Second))
	if err := second.ReadJSON(&message); err != nil {
		t.Fatal(err)
	}
	if len(message) != 5 || string(message[3]) != `"broadcast"` || string(message[4]) != `{"event":"message","payload":{"text":"hello"}}` {
		t.Fatalf("unexpected broadcast: %s", message)
	}
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
