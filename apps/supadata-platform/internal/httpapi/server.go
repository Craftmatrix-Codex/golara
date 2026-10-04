package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/auth"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/database"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

type Project = project.Project

type Registry interface {
	ListProjects(context.Context) ([]Project, error)
	CurrentProject(context.Context) (*Project, error)
	CreateProject(context.Context, string, string) (Project, error)
	SelectProject(context.Context, string) (Project, error)
}

type ProjectResolver interface {
	ResolveProject(context.Context, string) (Project, error)
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

type LogoutService interface {
	Logout(context.Context, string) error
}

type AdminUserService interface {
	ListUsers(context.Context, int, int) ([]auth.User, int, error)
}

type AdminUserDeleteService interface {
	DeleteUser(context.Context, string) error
}

type APIKeyConfig struct {
	Anon        string
	ServiceRole string
}
type ServerOptions struct {
	Token                string
	ControlPlaneUsername string
	ControlPlanePassword string
	AllowedOrigin        string
	Registry             Registry
	ProjectResolver      ProjectResolver
	DatabaseResolver     database.Resolver
	RequireProjectScope  bool
	Auth                 AuthService
	APIKeys              APIKeyConfig
	AuthSettings         AuthSettings
	REST                 http.Handler
	GraphQL              http.Handler
	Storage              http.Handler
	Realtime             http.Handler
	Functions            http.Handler
}

type Server struct {
	token                string
	controlPlaneUsername string
	controlPlanePassword string
	allowedOrigin        string
	registry             Registry
	projectResolver      ProjectResolver
	databaseResolver     database.Resolver
	requireProjectScope  bool
	auth                 AuthService
	apiKeys              APIKeyConfig
	authSettings         AuthSettings
	rest                 http.Handler
	graphql              http.Handler
	storage              http.Handler
	realtime             http.Handler
	functions            http.Handler
}

func NewServer(options ServerOptions) *Server {
	origin := options.AllowedOrigin
	if origin == "" {
		origin = "*"
	}
	return &Server{token: options.Token, controlPlaneUsername: options.ControlPlaneUsername, controlPlanePassword: options.ControlPlanePassword, allowedOrigin: origin, registry: options.Registry, projectResolver: options.ProjectResolver, databaseResolver: options.DatabaseResolver, requireProjectScope: options.RequireProjectScope, auth: options.Auth, apiKeys: options.APIKeys, authSettings: options.AuthSettings, rest: options.REST, graphql: options.GraphQL, storage: options.Storage, realtime: options.Realtime, functions: options.Functions}
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

func (s *Server) serveHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Access-Control-Allow-Headers", "content-type, authorization, apikey, x-client-info, x-graphql-authorization")
	response.Header().Set("Access-Control-Allow-Methods", "DELETE,GET,OPTIONS,POST,PATCH,PUT")
	response.Header().Set("Access-Control-Allow-Origin", s.allowedOrigin)

	if request.Method == http.MethodOptions {
		response.WriteHeader(http.StatusNoContent)
		return
	}
	projectGraphQLPath := strings.HasPrefix(request.URL.Path, "/api/projects/") && strings.HasSuffix(request.URL.Path, "/api/graphql")
	request = normalizeGraphQLProjectPath(request)
	request = normalizeProjectScopedSDKPath(request)
	if projectGraphQLPath && request.Header.Get("apikey") == "" && (request.Header.Get("Authorization") != "" || request.Header.Get("x-graphql-authorization") != "") {
		request.Header.Set("apikey", s.apiKeys.Anon)
	}
	if isProjectScopedPath(request.URL.Path) {
		scopedRequest, ok := s.withProjectScope(response, request)
		if !ok {
			return
		}
		request = scopedRequest
	}
	if request.URL.Path == "/graphql/v1" {
		if s.graphql == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "GraphQL service unavailable"})
			return
		}
		s.graphql.ServeHTTP(response, request)
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
	if strings.HasPrefix(request.URL.Path, "/functions/v1/") {
		if s.functions == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "functions service unavailable"})
			return
		}
		s.functions.ServeHTTP(response, request)
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
	if request.Method == http.MethodPost && request.URL.Path == "/auth/v1/logout" {
		if !s.hasAPIKey(request.Header.Get("apikey")) || !strings.HasPrefix(request.Header.Get("Authorization"), "Bearer ") {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		logoutService, ok := s.auth.(LogoutService)
		if !ok {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "auth storage unavailable"})
			return
		}
		accessToken := strings.TrimSpace(strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer "))
		if accessToken == "" {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if err := logoutService.Logout(request.Context(), accessToken); err != nil {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}
		writeJSON(response, http.StatusNoContent, nil)
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/auth/v1/admin/users" {
		if !s.isServiceRoleRequest(request) {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		adminService, ok := s.auth.(AdminUserService)
		if !ok {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "auth storage unavailable"})
			return
		}
		page, perPage, valid := pagination(request)
		if !valid {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid pagination"})
			return
		}
		users, total, err := adminService.ListUsers(request.Context(), page, perPage)
		if err != nil {
			writeJSON(response, http.StatusBadGateway, map[string]string{"error": "could not list users"})
			return
		}
		writeJSON(response, http.StatusOK, map[string]any{"users": users, "total": total})
		return
	}
	if request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, "/auth/v1/admin/users/") {
		if !s.isServiceRoleRequest(request) {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		deleteService, ok := s.auth.(AdminUserDeleteService)
		if !ok {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "auth storage unavailable"})
			return
		}
		userID := strings.TrimPrefix(request.URL.Path, "/auth/v1/admin/users/")
		if userID == "" || strings.Contains(userID, "/") {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
			return
		}
		if err := deleteService.DeleteUser(request.Context(), userID); err != nil {
			writeJSON(response, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		writeJSON(response, http.StatusNoContent, nil)
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
		isStudioLintPath(request.URL.Path) ||
		strings.HasPrefix(request.URL.Path, "/proxy") ||
		strings.HasPrefix(request.URL.Path, "/proxy-meta")
	if protected && !s.hasControlPlaneAuth(request.Header.Get("Authorization")) {
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
	case request.Method == http.MethodGet && isStudioLintPath(request.URL.Path):
		writeJSON(response, http.StatusOK, []any{})
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

func isProjectScopedPath(path string) bool {
	if strings.HasPrefix(path, "/rest/v1/") || strings.HasPrefix(path, "/storage/v1/") || strings.HasPrefix(path, "/realtime/v1/") || strings.HasPrefix(path, "/functions/v1/") || path == "/graphql/v1" {
		return true
	}
	return strings.HasPrefix(path, "/auth/v1/") && path != "/auth/v1/health"
}

func normalizeGraphQLProjectPath(request *http.Request) *http.Request {
	const prefix = "/api/projects/"
	const suffix = "/api/graphql"
	if !strings.HasPrefix(request.URL.Path, prefix) || !strings.HasSuffix(request.URL.Path, suffix) {
		return request
	}
	projectID := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, prefix), suffix)
	if projectID == "" || strings.Contains(projectID, "/") {
		return request
	}
	cloned := request.Clone(request.Context())
	cloned.URL.Path = "/graphql/v1"
	if cloned.Header.Get("X-Supadata-Project") == "" {
		cloned.Header.Set("X-Supadata-Project", projectID)
	}
	return cloned
}

func normalizeProjectScopedSDKPath(request *http.Request) *http.Request {
	const prefix = "/api/projects/"
	if !strings.HasPrefix(request.URL.Path, prefix) {
		return request
	}

	remainder := strings.TrimPrefix(request.URL.Path, prefix)
	separator := strings.IndexByte(remainder, '/')
	if separator <= 0 || separator == len(remainder)-1 {
		return request
	}
	projectID := remainder[:separator]
	sdkPath := "/" + remainder[separator+1:]
	for _, servicePrefix := range []string{"/rest/v1/", "/auth/v1/", "/storage/v1/", "/realtime/v1/", "/functions/v1/", "/graphql/v1"} {
		if !strings.HasPrefix(sdkPath, servicePrefix) {
			continue
		}
		cloned := request.Clone(request.Context())
		cloned.URL.Path = sdkPath
		if cloned.Header.Get("X-Supadata-Project") == "" {
			cloned.Header.Set("X-Supadata-Project", projectID)
		}
		return cloned
	}
	return request
}

func (s *Server) withProjectScope(response http.ResponseWriter, request *http.Request) (*http.Request, bool) {
	projectID := strings.TrimSpace(request.Header.Get("X-Supadata-Project"))
	if projectID == "" && s.projectResolver != nil {
		if hostResolver, ok := s.projectResolver.(interface {
			ResolveProjectHost(context.Context, string) (Project, error)
		}); ok {
			resolved, err := hostResolver.ResolveProjectHost(request.Context(), request.Host)
			if err == nil {
				return s.attachProjectScope(response, request, resolved)
			}
			if errors.Is(err, project.ErrNotFound) {
				writeJSON(response, http.StatusNotFound, map[string]string{"error": "project not found"})
				return nil, false
			}
		}
	}
	if projectID == "" {
		if s.requireProjectScope {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "project scope is required"})
			return nil, false
		}
		return request, true
	}
	if s.projectResolver == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "project resolver unavailable"})
		return nil, false
	}
	resolved, err := s.projectResolver.ResolveProject(request.Context(), projectID)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			writeJSON(response, http.StatusNotFound, map[string]string{"error": "project not found"})
			return nil, false
		}
		writeJSON(response, http.StatusBadGateway, map[string]string{"error": "project resolver failed"})
		return nil, false
	}
	return s.attachProjectScope(response, request, resolved)
}

func (s *Server) attachProjectScope(response http.ResponseWriter, request *http.Request, resolved Project) (*http.Request, bool) {
	ctx := project.WithScope(request.Context(), resolved)
	if s.databaseResolver == nil {
		return request.WithContext(ctx), true
	}
	databaseConnection, err := s.databaseResolver.Resolve(request.Context(), resolved)
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "project database unavailable"})
		return nil, false
	}
	return request.WithContext(database.WithConnection(ctx, databaseConnection)), true
}

func (s *Server) isServiceRoleRequest(request *http.Request) bool {
	configured := s.apiKeys.ServiceRole
	providedKey := request.Header.Get("apikey")
	authorization := request.Header.Get("Authorization")
	providedToken := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	return configured != "" && len(configured) == len(providedKey) && len(configured) == len(providedToken) &&
		subtle.ConstantTimeCompare([]byte(configured), []byte(providedKey)) == 1 &&
		subtle.ConstantTimeCompare([]byte(configured), []byte(providedToken)) == 1
}

func pagination(request *http.Request) (int, int, bool) {
	page, perPage := 1, 50
	var err error
	if value := request.URL.Query().Get("page"); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 {
			return 0, 0, false
		}
	}
	if value := request.URL.Query().Get("per_page"); value != "" {
		perPage, err = strconv.Atoi(value)
		if err != nil || perPage < 1 || perPage > 1000 {
			return 0, 0, false
		}
	}
	return page, perPage, true
}

func isStudioLintPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[0] == "api" && parts[1] == "platform" && parts[2] == "projects" && parts[4] == "run-lints" && parts[3] != ""
}

func (s *Server) hasControlPlaneAuth(header string) bool {
	if auth.HasValidBearerToken(s.token, header) {
		return true
	}
	if s.controlPlaneUsername == "" || s.controlPlanePassword == "" || !strings.HasPrefix(header, "Basic ") {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(header, "Basic ")))
	if err != nil {
		return false
	}
	expected := s.controlPlaneUsername + ":" + s.controlPlanePassword
	return len(decoded) == len(expected) && subtle.ConstantTimeCompare(decoded, []byte(expected)) == 1
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
