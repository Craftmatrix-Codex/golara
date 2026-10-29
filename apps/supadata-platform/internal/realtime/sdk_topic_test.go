package realtime

import (
	"github.com/gorilla/websocket"
	"testing"
)

func TestSDKPublicChannelNameDoesNotNeedAnExtraPublicPrefix(t *testing.T) {
	_, url := realtimeTestServer(t)
	connection, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	topic := "realtime:golara_sdk_contract"
	if err := connection.WriteJSON([]any{"1", "1", topic, "phx_join", map[string]any{"config": map[string]any{"private": false}}}); err != nil {
		t.Fatal(err)
	}
	assertReply(t, connection, "1", topic, "ok")
}

func TestSDKPrivateChannelCannotBeJoinedAsPublic(t *testing.T) {
	_, url := realtimeTestServer(t)
	connection, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	topic := "realtime:public:room"
	if err := connection.WriteJSON([]any{"1", "1", topic, "phx_join", map[string]any{"config": map[string]any{"private": true}}}); err != nil {
		t.Fatal(err)
	}
	assertReply(t, connection, "1", topic, "error")
}
