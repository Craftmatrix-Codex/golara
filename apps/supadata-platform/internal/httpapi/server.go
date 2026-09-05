package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/auth"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

type Project = project.Project

type Registry interface {
	ListProjects(context.Context) ([]Project, error)
	CurrentProject(context.Context) (*Project, error)
	CreateProject(context.Context, string, string) (Project, error)
	SelectProject(context.Context, string) (Project, error)
}

type AuthService interface {
	SignUp(context.Context, string, string, map[string]any) (auth.SessionResponse, error)
	SignIn(context.Context, string, string) (auth.SessionResponse, error)
	Refresh(context.Context, string) (auth.SessionResponse, error)
}

type AuthSettings struct {
	AnonymousUsers    bool
	EmailEnabled      bool
	PhoneEnabled      bool
	MailerAutoconfirm bool
	PhoneAutoconfirm  bool
	SMSProvider       string
	SAMLEnabled       bool
	DisableSignup     bool
}

type AccessTokenUserService interface {
	GetUserByAccessToken(context.Context, string) (auth.User, error)
}

type APIKeyConfig struct {
	Anon        string
	ServiceRole string
}
type ServerOptions struct {
	Token         string
	AllowedOrigin string
	Registry      Registry
	Auth          AuthService
	APIKeys       APIKeyConfig
	AuthSettings  AuthSettings
	REST          http.Handler
	Storage       http.Handler
	Realtime      http.Handler
}

type Server struct {
	token         string
	allowedOrigin string
	registry      Registry
	auth          AuthService
	apiKeys       APIKeyConfig
	authSettings  AuthSettings
	rest          http.Handler
	storage       http.Handler
	realtime      http.Handler
}

func NewServer(options ServerOptions) *Server {
	origin := options.AllowedOrigin
	if origin == "" {
		origin = "*"
	}
	return &Server{token: options.Token, allowedOrigin: origin, registry: options.Registry, auth: options.Auth, apiKeys: options.APIKeys, authSettings: options.AuthSettings, rest: options.REST, storage: options.Storage, realtime: options.Realtime}
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

func (s *Server) serveHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Access-Control-Allow-Headers", "content-type, authorization, apikey, x-client-info")
	response.Header().Set("Access-Control-Allow-Methods", "DELETE,GET,OPTIONS,POST,PATCH,PUT")
	response.Header().Set("Access-Control-Allow-Origin", s.allowedOrigin)

	if request.Method == http.MethodOptions {
		response.WriteHeader(http.StatusNoContent)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/rest/v1/") {
		if s.rest == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "REST service unavailable"})
			return
		}
		s.rest.ServeHTTP(response, request)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/storage/v1/") {
		if s.storage == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "storage service unavailable"})
			return
		}
		s.storage.ServeHTTP(response, request)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/realtime/v1/") {
		if s.realtime == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "realtime service unavailable"})
			return
		}
		s.realtime.ServeHTTP(response, request)
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/health" {
		writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/auth/v1/health" {
		writeJSON(response, http.StatusOK, map[string]string{
			"description": "Auth is a user registration and authentication API",
			"name":        "Auth",
			"version":     "",
		})
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/auth/v1/user" {
		if !s.hasAPIKey(request.Header.Get("apikey")) || !strings.HasPrefix(request.Header.Get("Authorization"), "Bearer ") {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		userService, ok := s.auth.(AccessTokenUserService)
		if !ok {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "auth storage unavailable"})
			return
		}
		accessToken := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		if accessToken == "" {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		user, err := userService.GetUserByAccessToken(request.Context(), accessToken)
		if err != nil {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}
		writeJSON(response, http.StatusOK, user)
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/auth/v1/settings" {
		if !s.hasAPIKey(request.Header.Get("apikey")) {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		settings := s.authSettings
		writeJSON(response, http.StatusOK, map[string]any{
			"external": map[string]bool{
				"anonymous_users": settings.AnonymousUsers,
				"email":           settings.EmailEnabled,
				"phone":           settings.PhoneEnabled,
			},
			"disable_signup":     settings.DisableSignup,
			"mailer_autoconfirm": settings.MailerAutoconfirm,
			"phone_autoconfirm":  settings.PhoneAutoconfirm,
			"sms_provider":       settings.SMSProvider,
			"saml_enabled":       settings.SAMLEnabled,
		})
		return
	}
	if (request.URL.Path == "/auth/v1/signup" || request.URL.Path == "/auth/v1/token") && !s.hasAPIKey(request.Header.Get("apikey")) {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if request.URL.Path == "/auth/v1/signup" && request.Method == http.MethodPost {
		s.handleSignup(response, request)
		return
	}
	if request.URL.Path == "/auth/v1/token" && request.Method == http.MethodPost {
		s.handleToken(response, request)
		return
	}

	protected := strings.HasPrefix(request.URL.Path, "/api/projects") ||
		strings.HasPrefix(request.URL.Path, "/proxy") ||
		strings.HasPrefix(request.URL.Path, "/proxy-meta")
	if protected && !auth.HasValidBearerToken(s.token, request.Header.Get("Authorization")) {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if s.registry == nil {
		writeJSON(response, http.StatusBadGateway, map[string]string{"error": "registry unavailable"})
		return
	}

	ctx := request.Context()
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/api/projects":
		projects, err := s.registry.ListProjects(ctx)
		if err != nil {
			writeJSON(response, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(response, http.StatusOK, map[string]any{"projects": projects})
	case request.Method == http.MethodGet && request.URL.Path == "/api/projects/current":
		project, err := s.registry.CurrentProject(ctx)
		if err != nil {
			writeJSON(response, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(response, http.StatusOK, map[string]any{"project": project})
	case request.Method == http.MethodPost && request.URL.Path == "/api/projects":
		var body struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		project, err := s.registry.CreateProject(ctx, strings.TrimSpace(body.Name), strings.TrimSpace(body.ID))
		if err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(response, http.StatusCreated, map[string]any{"project": project})
	case request.Method == http.MethodPost && strings.HasPrefix(request.URL.Path, "/api/projects/") && strings.HasSuffix(request.URL.Path, "/select"):
		id := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/api/projects/"), "/select")
		if id == "" || strings.Contains(id, "/") {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid project id"})
			return
		}
		project, err := s.registry.SelectProject(ctx, id)
		if err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		project.Current = true
		writeJSON(response, http.StatusOK, map[string]any{"project": project})
	default:
		writeJSON(response, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (s *Server) hasAPIKey(provided string) bool {
	if provided == "" {
		return false
	}
	for _, configured := range []string{s.apiKeys.Anon, s.apiKeys.ServiceRole} {
		if configured != "" && len(configured) == len(provided) && subtle.ConstantTimeCompare([]byte(configured), []byte(provided)) == 1 {
			return true
		}
	}
	return false
}

func (s *Server) handleSignup(response http.ResponseWriter, request *http.Request) {
	if s.authSettings.DisableSignup {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "signup is disabled"})
		return
	}
	if s.auth == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "auth storage unavailable"})
		return
	}
	var body struct {
		Email    string         `json:"email"`
		Password string         `json:"password"`
		Data     map[string]any `json:"data"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	result, err := s.auth.SignUp(request.Context(), body.Email, body.Password, body.Data)
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) handleToken(response http.ResponseWriter, request *http.Request) {
	if s.auth == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "auth storage unavailable"})
		return
	}
	grantType := request.URL.Query().Get("grant_type")
	if grantType == "refresh_token" {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		result, err := s.auth.Refresh(request.Context(), body.RefreshToken)
		if err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(response, http.StatusOK, result)
		return
	}
	if grantType != "password" {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "unsupported grant type"})
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	result, err := s.auth.SignIn(request.Context(), body.Email, body.Password)
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	if status != http.StatusNoContent {
		_ = json.NewEncoder(response).Encode(payload)
	}
}
