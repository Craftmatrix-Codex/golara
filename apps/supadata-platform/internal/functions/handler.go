package functions

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

var functionSlugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_-]{0,62}[a-z0-9])?$`)

type APIKeyConfig struct {
	Anon        string
	ServiceRole string
}

type Invocation struct {
	ProjectID string
	Slug      string
	Method    string
	RawQuery  string
	Headers   http.Header
	Body      io.Reader
}

type Result struct {
	Status  int
	Headers http.Header
	Body    []byte
}

type Executor interface {
	Invoke(context.Context, Invocation) (Result, error)
}

type jwtPolicy interface {
	RequiresJWT(context.Context, string, string) (bool, error)
}

type HandlerOptions struct {
	Executor  Executor
	APIKeys   APIKeyConfig
	JWTSecret []byte
}

type Handler struct {
	executor  Executor
	apiKeys   APIKeyConfig
	jwtSecret []byte
}

func NewHandler(options HandlerOptions) *Handler {
	return &Handler{executor: options.Executor, apiKeys: options.APIKeys, jwtSecret: append([]byte(nil), options.JWTSecret...)}
}

func (h *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if h.apiKeyRole(request.Header.Get("apikey")) == "" {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	const prefix = "/functions/v1/"
	if !strings.HasPrefix(request.URL.Path, prefix) {
		writeJSON(response, http.StatusNotFound, map[string]string{"error": "function route not found"})
		return
	}
	slug := strings.TrimPrefix(request.URL.Path, prefix)
	if !functionSlugPattern.MatchString(slug) {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid function slug"})
		return
	}
	scope, ok := project.ScopeFromContext(request.Context())
	if !ok || scope.ID == "" {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "project scope is required"})
		return
	}
	if h.executor == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "function runtime unavailable"})
		return
	}
	if policy, ok := h.executor.(jwtPolicy); ok {
		required, err := policy.RequiresJWT(request.Context(), scope.ID, slug)
		if IsNotFound(err) {
			writeJSON(response, http.StatusNotFound, map[string]string{"error": "function not found"})
			return
		}
		if err != nil {
			writeJSON(response, http.StatusBadGateway, map[string]string{"error": "function policy unavailable"})
			return
		}
		if required && !h.validJWT(request.Header.Get("Authorization"), scope.ID) {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "valid bearer token required"})
			return
		}
	}
	result, err := h.executor.Invoke(request.Context(), Invocation{
		ProjectID: scope.ID,
		Slug:      slug,
		Method:    request.Method,
		RawQuery:  request.URL.RawQuery,
		Headers:   request.Header.Clone(),
		Body:      request.Body,
	})
	if IsNotFound(err) {
		writeJSON(response, http.StatusNotFound, map[string]string{"error": "function not found"})
		return
	}
	if err != nil {
		writeJSON(response, http.StatusBadGateway, map[string]string{"error": "function invocation failed"})
		return
	}
	for name, values := range result.Headers {
		for _, value := range values {
			response.Header().Add(name, value)
		}
	}
	status := result.Status
	if status < 100 || status > 599 {
		status = http.StatusOK
	}
	response.WriteHeader(status)
	_, _ = response.Write(result.Body)
}

func (h *Handler) validJWT(authorization, projectID string) bool {
	if len(h.jwtSecret) == 0 || !strings.HasPrefix(authorization, "Bearer ") {
		return false
	}
	claims, err := jwt.VerifyHS256(strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")), h.jwtSecret, jwt.ValidationOptions{Now: time.Now()})
	if err != nil {
		return false
	}
	return claims.ProjectID == "" || claims.ProjectID == projectID
}

func (h *Handler) apiKeyRole(provided string) string {
	for _, candidate := range []struct {
		key  string
		role string
	}{{h.apiKeys.ServiceRole, "service_role"}, {h.apiKeys.Anon, "anon"}} {
		if candidate.key != "" && len(candidate.key) == len(provided) && subtle.ConstantTimeCompare([]byte(candidate.key), []byte(provided)) == 1 {
			return candidate.role
		}
	}
	return ""
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}
