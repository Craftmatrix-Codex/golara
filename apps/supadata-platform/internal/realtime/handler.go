package realtime

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

type APIKeyConfig struct {
	Anon        string
	ServiceRole string
}

type ChangeSubscription struct {
	Event  string `json:"event"`
	Schema string `json:"schema"`
	Table  string `json:"table"`
	Filter string `json:"filter,omitempty"`
}

type ChangeEvent struct {
	Schema    string         `json:"schema"`
	Table     string         `json:"table"`
	Event     string         `json:"type"`
	Record    map[string]any `json:"record,omitempty"`
	OldRecord map[string]any `json:"old_record,omitempty"`
}

type ChangeSource interface {
	Subscribe(context.Context, string, ChangeSubscription, func(ChangeEvent)) (func(), error)
}

type HandlerOptions struct {
	APIKeys       APIKeyConfig
	JWTSecret     []byte
	Issuer        string
	Audience      string
	AllowedOrigin string
	ChangeSource  ChangeSource
}

type Handler struct {
	apiKeys       APIKeyConfig
	jwtSecret     []byte
	issuer        string
	audience      string
	allowedOrigin string
	changeSource  ChangeSource
	upgrader      websocket.Upgrader
	hubMu         sync.RWMutex
	topics        map[string]map[*realtimeClient]struct{}
}

type realtimeClient struct {
	connection    *websocket.Conn
	role          string
	writeMu       sync.Mutex
	unsubscribers []func()
}

func NewHandler(options HandlerOptions) *Handler {
	allowedOrigin := options.AllowedOrigin
	return &Handler{
		apiKeys:       options.APIKeys,
		jwtSecret:     append([]byte(nil), options.JWTSecret...),
		issuer:        options.Issuer,
		audience:      options.Audience,
		allowedOrigin: allowedOrigin,
		changeSource:  options.ChangeSource,
		topics:        make(map[string]map[*realtimeClient]struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4 << 10,
			WriteBufferSize: 4 << 10,
			CheckOrigin: func(request *http.Request) bool {
				origin := request.Header.Get("Origin")
				return origin == "" || allowedOrigin == "" || allowedOrigin == "*" || subtle.ConstantTimeCompare([]byte(origin), []byte(allowedOrigin)) == 1
			},
		},
	}
}

func (h *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/realtime/v1/websocket" {
		writeJSON(response, http.StatusNotFound, map[string]string{"error": "realtime route not found"})
		return
	}
	role := h.apiKeyRole(request.URL.Query().Get("apikey"), request.Header.Get("apikey"))
	if role == "" {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if err := h.validateAccessToken(request); err != nil {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}
	connection, err := h.upgrader.Upgrade(response, request, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	client := &realtimeClient{connection: connection, role: role}
	defer h.removeClient(client)
	connection.SetReadLimit(1 << 20)
	joinedTopics := make(map[string]struct{})
	for {
		var message []json.RawMessage
		if err := connection.ReadJSON(&message); err != nil {
			return
		}
		if err := h.handleMessage(request.Context(), client, message, joinedTopics); err != nil {
			return
		}
	}
}

func (h *Handler) handleMessage(ctx context.Context, client *realtimeClient, message []json.RawMessage, joinedTopics map[string]struct{}) error {
	if len(message) != 5 {
		return errors.New("invalid realtime message")
	}
	var joinReference, reference, topic, event string
	if string(message[0]) != "null" && json.Unmarshal(message[0], &joinReference) != nil {
		return errors.New("invalid join reference")
	}
	if json.Unmarshal(message[1], &reference) != nil || json.Unmarshal(message[2], &topic) != nil || json.Unmarshal(message[3], &event) != nil {
		return errors.New("invalid realtime message fields")
	}
	switch {
	case event == "heartbeat" && topic == "phoenix":
		return client.writeReply(joinReference, reference, topic, "ok", map[string]any{})
	case event == "phx_join":
		if !strings.HasPrefix(topic, "realtime:public:") || strings.TrimPrefix(topic, "realtime:public:") == "" {
			return client.writeReply(joinReference, reference, topic, "error", map[string]string{"reason": "unauthorized topic"})
		}
		topicKey := ProjectTopicKey(ctx, topic)
		joinedTopics[topicKey] = struct{}{}
		h.addClient(topicKey, client)
		if err := h.subscribePostgresChanges(ctx, client, topic, message[4]); err != nil {
			delete(joinedTopics, topicKey)
			h.removeClientFromTopic(topicKey, client)
			return client.writeReply(joinReference, reference, topic, "error", map[string]string{"reason": "postgres changes unavailable"})
		}
		return client.writeReply(joinReference, reference, topic, "ok", map[string]any{})
	case event == "phx_leave":
		topicKey := ProjectTopicKey(ctx, topic)
		delete(joinedTopics, topicKey)
		h.removeClientFromTopic(topicKey, client)
		if err := client.writeReply(joinReference, reference, topic, "ok", map[string]any{}); err != nil {
			return err
		}
		return errors.New("connection left")
	case event == "broadcast":
		topicKey := ProjectTopicKey(ctx, topic)
		if _, joined := joinedTopics[topicKey]; !joined {
			return client.writeReply(joinReference, reference, topic, "error", map[string]string{"reason": "not joined"})
		}
		h.broadcast(topicKey, []any{nil, reference, topic, "broadcast", message[4]})
		return client.writeReply(joinReference, reference, topic, "ok", map[string]any{})
	default:
		return client.writeReply(joinReference, reference, topic, "error", map[string]string{"reason": "unsupported event"})
	}
}

func (h *Handler) subscribePostgresChanges(ctx context.Context, client *realtimeClient, topic string, raw json.RawMessage) error {
	var payload struct {
		Config struct {
			PostgresChanges []ChangeSubscription `json:"postgres_changes"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	if len(payload.Config.PostgresChanges) == 0 {
		return nil
	}
	if client.role != "service_role" {
		return errors.New("postgres changes require service role")
	}
	if h.changeSource == nil {
		return errors.New("change source unavailable")
	}
	projectID := "default"
	if scope, ok := project.ScopeFromContext(ctx); ok && scope.ID != "" {
		projectID = scope.ID
	}
	start := len(client.unsubscribers)
	for _, subscription := range payload.Config.PostgresChanges {
		subscription.Event = strings.ToUpper(subscription.Event)
		unsubscribe, err := h.changeSource.Subscribe(ctx, projectID, subscription, func(event ChangeEvent) {
			_ = client.writeJSON([]any{nil, nil, topic, "postgres_changes", map[string]any{"ids": []int{1}, "data": event}})
		})
		if err != nil {
			for _, stop := range client.unsubscribers[start:] {
				stop()
			}
			client.unsubscribers = client.unsubscribers[:start]
			return err
		}
		client.unsubscribers = append(client.unsubscribers, unsubscribe)
	}
	return nil
}

func (h *Handler) validateAccessToken(request *http.Request) error {
	if len(h.jwtSecret) == 0 {
		return nil
	}
	token := request.URL.Query().Get("access_token")
	if token == "" {
		authorization := request.Header.Get("Authorization")
		if strings.HasPrefix(authorization, "Bearer ") {
			token = strings.TrimPrefix(authorization, "Bearer ")
		}
	}
	if token == "" {
		return nil
	}
	claims, err := jwt.VerifyHS256(token, h.jwtSecret, jwt.ValidationOptions{Now: time.Now(), Issuer: h.issuer, Audience: h.audience})
	if err != nil {
		return err
	}
	if scope, ok := project.ScopeFromContext(request.Context()); ok && claims.ProjectID != "" && claims.ProjectID != scope.ID {
		return errors.New("project token mismatch")
	}
	return nil
}

func (h *Handler) apiKeyRole(queryKey, headerKey string) string {
	for _, provided := range []string{queryKey, headerKey} {
		if provided == "" {
			continue
		}
		for _, candidate := range []struct {
			key  string
			role string
		}{
			{h.apiKeys.ServiceRole, "service_role"},
			{h.apiKeys.Anon, "anon"},
		} {
			if candidate.key != "" && len(candidate.key) == len(provided) && subtle.ConstantTimeCompare([]byte(candidate.key), []byte(provided)) == 1 {
				return candidate.role
			}
		}
	}
	return ""
}

func (c *realtimeClient) writeJSON(payload any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.connection.WriteJSON(payload)
}

func (c *realtimeClient) writeReply(joinReference, reference, topic, status string, response any) error {
	return c.writeJSON([]any{joinReference, reference, topic, "phx_reply", map[string]any{"status": status, "response": response}})
}

func (h *Handler) addClient(topic string, client *realtimeClient) {
	h.hubMu.Lock()
	defer h.hubMu.Unlock()
	if h.topics[topic] == nil {
		h.topics[topic] = make(map[*realtimeClient]struct{})
	}
	h.topics[topic][client] = struct{}{}
}

func (h *Handler) removeClientFromTopic(topic string, client *realtimeClient) {
	h.hubMu.Lock()
	defer h.hubMu.Unlock()
	delete(h.topics[topic], client)
	if len(h.topics[topic]) == 0 {
		delete(h.topics, topic)
	}
}

func (h *Handler) removeClient(client *realtimeClient) {
	for _, unsubscribe := range client.unsubscribers {
		unsubscribe()
	}
	client.unsubscribers = nil
	h.hubMu.Lock()
	defer h.hubMu.Unlock()
	for topic, clients := range h.topics {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.topics, topic)
		}
	}
}

func (h *Handler) broadcast(topic string, payload any) {
	h.hubMu.RLock()
	clients := make([]*realtimeClient, 0, len(h.topics[topic]))
	for client := range h.topics[topic] {
		clients = append(clients, client)
	}
	h.hubMu.RUnlock()
	for _, client := range clients {
		_ = client.writeJSON(payload)
	}
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}
