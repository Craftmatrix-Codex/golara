package graphql

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	platformjwt "github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

type APIKeyConfig struct {
	Anon        string
	ServiceRole string
}

type HandlerOptions struct {
	Database  *sql.DB
	APIKeys   APIKeyConfig
	JWTSecret []byte
	Issuer    string
	Audience  string
}

type Handler struct {
	database  *sql.DB
	apiKeys   APIKeyConfig
	jwtSecret []byte
	issuer    string
	audience  string
}

type requestBody struct {
	Query         string          `json:"query"`
	Variables     json.RawMessage `json:"variables"`
	OperationName string          `json:"operationName"`
	Extensions    json.RawMessage `json:"extensions"`
}

func NewHandler(options HandlerOptions) *Handler {
	return &Handler{
		database:  options.Database,
		apiKeys:   options.APIKeys,
		jwtSecret: append([]byte(nil), options.JWTSecret...),
		issuer:    options.Issuer,
		audience:  options.Audience,
	}
}

func (h *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeJSON(response, http.StatusMethodNotAllowed, map[string]any{"errors": []map[string]string{{"message": "method not allowed"}}})
		return
	}
	role := h.apiKeyRole(request.Header.Get("apikey"))
	if role == "" {
		writeJSON(response, http.StatusUnauthorized, map[string]any{"errors": []map[string]string{{"message": "invalid API key"}}})
		return
	}
	claims, err := h.accessClaims(request, role)
	if err != nil {
		writeJSON(response, http.StatusUnauthorized, map[string]any{"errors": []map[string]string{{"message": "invalid access token"}}})
		return
	}
	var body requestBody
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 2<<20))
	if err := decoder.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(response, http.StatusBadRequest, map[string]any{"errors": []map[string]string{{"message": "invalid GraphQL request"}}})
		return
	}
	if strings.TrimSpace(body.Query) == "" {
		writeJSON(response, http.StatusBadRequest, map[string]any{"errors": []map[string]string{{"message": "query is required"}}})
		return
	}
	if h.database == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]any{"errors": []map[string]string{{"message": "GraphQL database unavailable"}}})
		return
	}

	variables := body.Variables
	if len(variables) == 0 {
		variables = json.RawMessage(`{}`)
	}
	extensions := body.Extensions
	if len(extensions) == 0 {
		extensions = json.RawMessage(`null`)
	}

	tx, err := h.database.BeginTx(request.Context(), nil)
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]any{"errors": []map[string]string{{"message": "GraphQL database unavailable"}}})
		return
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()
	var schemaVersion int64
	_, _ = tx.ExecContext(request.Context(), `SAVEPOINT graphql_schema_refresh`)
	if _, err := tx.ExecContext(request.Context(), `SELECT graphql.increment_schema_version()`); err != nil {
		_, _ = tx.ExecContext(request.Context(), `ROLLBACK TO SAVEPOINT graphql_schema_refresh`)
	}
	_, _ = tx.ExecContext(request.Context(), `RELEASE SAVEPOINT graphql_schema_refresh`)
	if err := tx.QueryRowContext(request.Context(), `SELECT last_value FROM graphql.seq_schema_version`).Scan(&schemaVersion); err != nil {
		writeJSON(response, http.StatusBadGateway, map[string]any{"errors": []map[string]string{{"message": "GraphQL schema lookup failed"}}})
		return
	}
	if err := setRequestRole(request.Context(), tx, claims, role); err != nil {
		writeJSON(response, http.StatusBadGateway, map[string]any{"errors": []map[string]string{{"message": "GraphQL role setup failed"}}})
		return
	}

	operationName := any(nil)
	if strings.TrimSpace(body.OperationName) != "" {
		operationName = body.OperationName
	}

	var result []byte
	resolveQuery := fmt.Sprintf(`SELECT graphql.resolve($1, $2::jsonb, $3, $4::jsonb) /* schema_version=%d */`, schemaVersion)
	err = tx.QueryRowContext(request.Context(),
		resolveQuery,
		body.Query, string(variables), operationName, string(extensions),
	).Scan(&result)
	if err != nil {
		writeJSON(response, http.StatusBadGateway, map[string]any{"errors": []map[string]string{{"message": "GraphQL execution failed"}}})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(response, http.StatusBadGateway, map[string]any{"errors": []map[string]string{{"message": "GraphQL execution failed"}}})
		return
	}
	rollback = false
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(result)
}

func setRequestRole(ctx context.Context, tx *sql.Tx, claims platformjwt.Claims, apiKeyRole string) error {
	role := claims.Role
	if role == "" {
		role = apiKeyRole
	}
	if role != "anon" && role != "authenticated" && role != "service_role" {
		return errors.New("unsupported database role")
	}
	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE "`+role+`"`); err != nil {
		return err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return err
	}
	for key, value := range map[string]string{
		"request.jwt.claims":     string(claimsJSON),
		"request.jwt.claim.role": role,
		"request.jwt.claim.sub":  claims.Subject,
	} {
		var ignored string
		if err := tx.QueryRowContext(ctx, "select set_config($1, $2, true)", key, value).Scan(&ignored); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) accessClaims(request *http.Request, apiKeyRole string) (platformjwt.Claims, error) {
	authorization := request.Header.Get("x-graphql-authorization")
	if authorization == "" {
		authorization = request.Header.Get("Authorization")
	}
	if authorization == "" {
		return platformjwt.Claims{Role: apiKeyRole, Audience: h.audience}, nil
	}
	if !strings.HasPrefix(authorization, "Bearer ") || len(h.jwtSecret) == 0 {
		return platformjwt.Claims{}, errors.New("invalid access token")
	}
	claims, err := platformjwt.VerifyHS256(strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")), h.jwtSecret, platformjwt.ValidationOptions{Now: time.Now(), Issuer: h.issuer, Audience: h.audience})
	if err != nil {
		return platformjwt.Claims{}, err
	}
	if scope, ok := project.ScopeFromContext(request.Context()); ok && claims.ProjectID != "" && claims.ProjectID != scope.ID {
		return platformjwt.Claims{}, errors.New("project token mismatch")
	}
	if apiKeyRole != "service_role" && claims.Role == "service_role" {
		return platformjwt.Claims{}, errors.New("privilege escalation")
	}
	if claims.Role == "" {
		claims.Role = "authenticated"
	}
	return claims, nil
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
