package rest

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/database"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

type APIKeyConfig struct {
	Anon        string
	ServiceRole string
}

type HandlerOptions struct {
	Schema    string
	APIKeys   APIKeyConfig
	JWTSecret []byte
	Issuer    string
	Audience  string
}

type Queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type Handler struct {
	database  *sql.DB
	schema    string
	apiKeys   APIKeyConfig
	jwtSecret []byte
	issuer    string
	audience  string
}

func NewHandler(database *sql.DB, options HandlerOptions) *Handler {
	schema := options.Schema
	if schema == "" {
		schema = "public"
	}
	return &Handler{database: database, schema: schema, apiKeys: options.APIKeys, jwtSecret: append([]byte(nil), options.JWTSecret...), issuer: options.Issuer, audience: options.Audience}
}

func (h *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead && request.Method != http.MethodPost && request.Method != http.MethodPatch && request.Method != http.MethodDelete {
		writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !h.hasAPIKey(request.Header.Get("apikey")) {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	claims, err := h.accessClaims(request)
	if err != nil {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}
	if request.Method == http.MethodPost {
		if strings.HasPrefix(request.URL.Path, "/rest/v1/rpc/") {
			h.handleRPC(response, request, claims)
			return
		}
		h.handleInsert(response, request, claims)
		return
	}
	if request.Method == http.MethodPatch || request.Method == http.MethodDelete {
		h.handleMutation(response, request, claims)
		return
	}
	if h.databaseForContext(request.Context()) == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable"})
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/rest/v1/")
	if path == "" || strings.Contains(path, "/") {
		writeJSON(response, http.StatusNotFound, map[string]string{"error": "table not found"})
		return
	}
	schema := request.Header.Get("Accept-Profile")
	if schema == "" {
		schema = h.schema
	}
	query, err := BuildSelectQuery(schema, path, request.URL.Query())
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if request.Method == http.MethodHead {
		h.handleHead(response, request, claims, schema, path)
		return
	}
	result, err := h.queryRows(request.Context(), claims, query, wantsSingular(request))
	if err != nil {
		writeQueryError(response, err, "database query failed")
		return
	}
	response.Header().Set("Content-Range", contentRange(len(result)))
	if strings.Contains(request.Header.Get("Prefer"), "count=") {
		countQuery, err := BuildCountQuery(schema, path, request.URL.Query())
		if err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid count request"})
			return
		}
		counts, err := h.queryRows(request.Context(), claims, countQuery, false)
		if err != nil || len(counts) != 1 {
			writeJSON(response, http.StatusInternalServerError, map[string]string{"error": "count query failed"})
			return
		}
		total := fmt.Sprint(counts[0]["count"])
		rangeValue := "*/" + total
		if len(result) > 0 {
			offset, _ := strconv.Atoi(request.URL.Query().Get("offset"))
			rangeValue = strconv.Itoa(offset) + "-" + strconv.Itoa(offset+len(result)-1) + "/" + total
		}
		response.Header().Set("Content-Range", rangeValue)
	}
	writeRESTResult(response, request, http.StatusOK, result)
}

func (h *Handler) handleRPC(response http.ResponseWriter, request *http.Request, claims jwt.Claims) {
	if h.databaseForContext(request.Context()) == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable"})
		return
	}
	function := strings.TrimPrefix(request.URL.Path, "/rest/v1/rpc/")
	if function == "" || strings.Contains(function, "/") {
		writeJSON(response, http.StatusNotFound, map[string]string{"error": "function not found"})
		return
	}
	var arguments map[string]any
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 10<<20))
	if err := decoder.Decode(&arguments); err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid RPC body"})
		return
	}
	schema := request.Header.Get("Content-Profile")
	if schema == "" {
		schema = h.schema
	}
	query, err := BuildRPCQuery(schema, function, arguments)
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := h.queryRows(request.Context(), claims, query, wantsSingular(request))
	if err != nil {
		writeQueryError(response, err, "RPC execution failed")
		return
	}
	writeRESTResult(response, request, http.StatusOK, result)
}

func (h *Handler) databaseForContext(ctx context.Context) *sql.DB {
	if databaseConnection, ok := database.ConnectionFromContext(ctx); ok {
		return databaseConnection
	}
	return h.database
}

func (h *Handler) queryRows(ctx context.Context, claims jwt.Claims, query Query, singular bool) ([]map[string]any, error) {
	databaseConnection := h.databaseForContext(ctx)
	if databaseConnection == nil {
		return nil, errors.New("database unavailable")
	}
	tx, err := databaseConnection.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return nil, err
	}
	settings := []struct {
		key   string
		value string
	}{
		{"request.jwt.claims", string(claimsJSON)},
		{"request.jwt.claim.sub", claims.Subject},
		{"request.jwt.claim.role", claims.Role},
	}
	if scope, ok := project.ScopeFromContext(ctx); ok {
		settings = append(settings, struct {
			key   string
			value string
		}{"request.jwt.claim.project_id", scope.ID})
	}
	for _, setting := range settings {
		var ignored string
		if err := tx.QueryRowContext(ctx, "select set_config($1, $2, true)", setting.key, setting.value).Scan(&ignored); err != nil {
			return nil, err
		}
	}
	role := claims.Role
	if role == "" {
		role = "authenticated"
	}
	if role != "anon" && role != "authenticated" && role != "service_role" {
		return nil, errors.New("unsupported database role")
	}
	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE "`+role+`"`); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, query.SQL, query.Args...)
	if err != nil {
		return nil, err
	}
	result, err := rowsToJSON(rows)
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if singular && len(result) != 1 {
		return nil, &singularResultError{Count: len(result)}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	rollback = false
	return result, nil
}

func (h *Handler) handleMutation(response http.ResponseWriter, request *http.Request, claims jwt.Claims) {
	if h.databaseForContext(request.Context()) == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable"})
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/rest/v1/")
	if path == "" || strings.Contains(path, "/") {
		writeJSON(response, http.StatusNotFound, map[string]string{"error": "table not found"})
		return
	}
	schema := request.Header.Get("Content-Profile")
	if schema == "" {
		schema = h.schema
	}
	var query Query
	var err error
	switch request.Method {
	case http.MethodPatch:
		var updates map[string]any
		decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 10<<20))
		if err := decoder.Decode(&updates); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid update body"})
			return
		}
		query, err = BuildUpdateQuery(schema, path, updates, request.URL.Query())
	case http.MethodDelete:
		query, err = BuildDeleteQuery(schema, path, request.URL.Query())
	}
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := h.queryRows(request.Context(), claims, query, wantsSingular(request))
	if err != nil {
		writeQueryError(response, err, "database mutation failed")
		return
	}
	if strings.Contains(request.Header.Get("Prefer"), "return=representation") {
		writeRESTResult(response, request, http.StatusOK, result)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleInsert(response http.ResponseWriter, request *http.Request, claims jwt.Claims) {
	if h.databaseForContext(request.Context()) == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable"})
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/rest/v1/")
	if path == "" || strings.Contains(path, "/") {
		writeJSON(response, http.StatusNotFound, map[string]string{"error": "table not found"})
		return
	}
	schema := request.Header.Get("Content-Profile")
	if schema == "" {
		schema = h.schema
	}
	var payload json.RawMessage
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 10<<20))
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	var rows []map[string]any
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid insert body"})
			return
		}
	} else {
		var row map[string]any
		if err := json.Unmarshal(trimmed, &row); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid insert body"})
			return
		}
		rows = []map[string]any{row}
	}
	var query Query
	var err error
	if request.URL.Query().Get("on_conflict") != "" || strings.Contains(request.Header.Get("Prefer"), "resolution=merge-duplicates") {
		query, err = BuildUpsertQuery(schema, path, rows, request.URL.Query().Get("on_conflict"))
	} else {
		query, err = BuildInsertQuery(schema, path, rows)
	}
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := h.queryRows(request.Context(), claims, query, wantsSingular(request))
	if err != nil {
		writeQueryError(response, err, "database insert failed")
		return
	}
	if strings.Contains(request.Header.Get("Prefer"), "return=representation") {
		writeRESTResult(response, request, http.StatusCreated, result)
		return
	}
	response.WriteHeader(http.StatusCreated)
}

func (h *Handler) accessClaims(request *http.Request) (jwt.Claims, error) {
	role := h.apiKeyRole(request.Header.Get("apikey"))
	if role == "" {
		return jwt.Claims{}, errors.New("invalid API key")
	}
	authorization := request.Header.Get("Authorization")
	if authorization == "" {
		return jwt.Claims{Role: role, Audience: h.audience}, nil
	}
	if strings.HasPrefix(authorization, "Bearer ") && h.apiKeyRole(strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))) == role {
		return jwt.Claims{Role: role, Audience: h.audience}, nil
	}
	if !strings.HasPrefix(authorization, "Bearer ") || len(h.jwtSecret) == 0 {
		return jwt.Claims{}, errors.New("invalid access token")
	}
	token := strings.TrimPrefix(authorization, "Bearer ")
	if token == "" {
		return jwt.Claims{}, errors.New("invalid access token")
	}
	claims, err := jwt.VerifyHS256(token, h.jwtSecret, jwt.ValidationOptions{Now: time.Now(), Issuer: h.issuer, Audience: h.audience})
	if err != nil {
		return jwt.Claims{}, err
	}
	if scope, ok := project.ScopeFromContext(request.Context()); ok && claims.ProjectID != "" && claims.ProjectID != scope.ID {
		return jwt.Claims{}, errors.New("project token mismatch")
	}
	if role != "service_role" && claims.Role == "service_role" {
		return jwt.Claims{}, errors.New("privilege escalation")
	}
	if claims.Role == "" {
		claims.Role = "authenticated"
	}
	return claims, nil
}

func (h *Handler) apiKeyRole(provided string) string {
	if provided == "" {
		return ""
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
	return ""
}

func (h *Handler) hasAPIKey(provided string) bool {
	return h.apiKeyRole(provided) != ""
}

func rowsToJSON(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(columns))
		for index, column := range columns {
			row[column] = normalizeValue(values[index])
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeValue(value any) any {
	bytes, ok := value.([]byte)
	if !ok {
		return value
	}
	if json.Valid(bytes) {
		return json.RawMessage(bytes)
	}
	return string(bytes)
}

type singularResultError struct{ Count int }

func (e *singularResultError) Error() string { return "result is not singular" }

func wantsSingular(request *http.Request) bool {
	return strings.Contains(request.Header.Get("Accept"), "application/vnd.pgrst.object+json")
}

func writeQueryError(response http.ResponseWriter, err error, fallback string) {
	var singular *singularResultError
	if errors.As(err, &singular) {
		writeJSON(response, http.StatusNotAcceptable, map[string]any{"code": "PGRST116", "message": "Cannot coerce the result to a single JSON object", "details": fmt.Sprintf("The result contains %d rows", singular.Count), "hint": nil})
		return
	}
	writeJSON(response, http.StatusInternalServerError, map[string]string{"error": fallback})
}

func (h *Handler) handleHead(response http.ResponseWriter, request *http.Request, claims jwt.Claims, schema, table string) {
	query, err := BuildCountQuery(schema, table, request.URL.Query())
	if err != nil {
		writeQueryError(response, err, "invalid count request")
		return
	}
	counts, err := h.queryRows(request.Context(), claims, query, false)
	if err != nil || len(counts) != 1 {
		writeQueryError(response, err, "count query failed")
		return
	}
	total, err := strconv.Atoi(fmt.Sprint(counts[0]["count"]))
	if err != nil || total < 0 {
		writeQueryError(response, err, "invalid count result")
		return
	}
	offset, _ := strconv.Atoi(request.URL.Query().Get("offset"))
	selected := total - offset
	if selected < 0 {
		selected = 0
	}
	if raw := request.URL.Query().Get("limit"); raw != "" {
		limit, _ := strconv.Atoi(raw)
		if limit < selected {
			selected = limit
		}
	}
	denominator := "*"
	if strings.Contains(request.Header.Get("Prefer"), "count=") {
		denominator = strconv.Itoa(total)
	}
	rangeValue := "*/" + denominator
	if selected > 0 {
		rangeValue = strconv.Itoa(offset) + "-" + strconv.Itoa(offset+selected-1) + "/" + denominator
	}
	response.Header().Set("Content-Range", rangeValue)
	if wantsSingular(request) && selected != 1 {
		writeQueryError(response, &singularResultError{Count: selected}, "result is not singular")
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
}

func writeRESTResult(response http.ResponseWriter, request *http.Request, status int, result []map[string]any) {
	var payload any = result
	contentType := "application/json"
	if strings.Contains(request.Header.Get("Accept"), "application/vnd.pgrst.object+json") {
		if len(result) != 1 {
			writeJSON(response, http.StatusNotAcceptable, map[string]any{"code": "PGRST116", "message": "Cannot coerce the result to a single JSON object", "details": fmt.Sprintf("The result contains %d rows", len(result)), "hint": nil})
			return
		}
		payload = result[0]
		contentType = "application/vnd.pgrst.object+json"
	}
	response.Header().Set("Content-Type", contentType)
	response.WriteHeader(status)
	if request.Method != http.MethodHead {
		_ = json.NewEncoder(response).Encode(payload)
	}
}

func contentRange(count int) string {
	if count == 0 {
		return "*/0"
	}
	return "0-" + itoa(count-1) + "/*"
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	if err := json.NewEncoder(response).Encode(payload); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		return
	}
}
