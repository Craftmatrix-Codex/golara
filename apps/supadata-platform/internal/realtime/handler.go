package realtime

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
)

type APIKeyConfig struct {
	Anon        string
	ServiceRole string
}

type HandlerOptions struct {
	APIKeys       APIKeyConfig
	JWTSecret     []byte
	Issuer        string
	Audience      string
	AllowedOrigin string
}

type Handler struct {
	apiKeys       APIKeyConfig
	jwtSecret     []byte
	issuer        string
	audience      string
	allowedOrigin string
	upgrader      websocket.Upgrader
}

func NewHandler(options HandlerOptions) *Handler {
	allowedOrigin := options.AllowedOrigin
	return &Handler{
		apiKeys:       options.APIKeys,
		jwtSecret:     append([]byte(nil), options.JWTSecret...),
		issuer:        options.Issuer,
		audience:      options.Audience,
		allowedOrigin: allowedOrigin,
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
	if h.apiKeyRole(request.URL.Query().Get("apikey"), request.Header.Get("apikey")) == "" {
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
	connection.SetReadLimit(1 << 20)
	for {
		var message []json.RawMessage
		if err := connection.ReadJSON(&message); err != nil {
			return
		}
		if err := h.handleMessage(connection, message); err != nil {
			return
		}
	}
}

func (h *Handler) handleMessage(connection *websocket.Conn, message []json.RawMessage) error {
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
		return writeReply(connection, joinReference, reference, topic, "ok", map[string]any{})
	case event == "phx_join":
		if !strings.HasPrefix(topic, "realtime:public:") || strings.TrimPrefix(topic, "realtime:public:") == "" {
			return writeReply(connection, joinReference, reference, topic, "error", map[string]string{"reason": "unauthorized topic"})
		}
		return writeReply(connection, joinReference, reference, topic, "ok", map[string]any{})
	case event == "phx_leave":
		if err := writeReply(connection, joinReference, reference, topic, "ok", map[string]any{}); err != nil {
			return err
		}
		return errors.New("connection left")
	default:
		return writeReply(connection, joinReference, reference, topic, "error", map[string]string{"reason": "unsupported event"})
	}
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
	_, err := jwt.VerifyHS256(token, h.jwtSecret, jwt.ValidationOptions{Now: time.Now(), Issuer: h.issuer, Audience: h.audience})
	return err
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

func writeReply(connection *websocket.Conn, joinReference, reference, topic, status string, response any) error {
	return connection.WriteJSON([]any{joinReference, reference, topic, "phx_reply", map[string]any{"status": status, "response": response}})
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}
