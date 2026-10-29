package rest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

func TestProjectTokenCannotCrossProjectBoundary(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	secret := []byte("jwt-secret")
	token := signedProjectToken(t, secret, "alpha")
	handler := NewHandler(database, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}, JWTSecret: secret})

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT\s+set_config\('role'`).WithArgs("authenticated", sqlmock.AnyArg(), "user-id", "alpha").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "public"."todos"`)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	alphaRequest := httptest.NewRequest(http.MethodGet, "/rest/v1/todos", nil)
	alphaRequest.Header.Set("apikey", "anon-key")
	alphaRequest.Header.Set("Authorization", "Bearer "+token)
	alphaRequest = alphaRequest.WithContext(project.WithScope(alphaRequest.Context(), project.Project{ID: "alpha"}))
	alphaResponse := httptest.NewRecorder()
	handler.ServeHTTP(alphaResponse, alphaRequest)
	if alphaResponse.Code != http.StatusOK {
		t.Fatalf("alpha token status = %d, want 200: %s", alphaResponse.Code, alphaResponse.Body.String())
	}

	betaRequest := httptest.NewRequest(http.MethodGet, "/rest/v1/todos", nil)
	betaRequest.Header.Set("apikey", "anon-key")
	betaRequest.Header.Set("Authorization", "Bearer "+token)
	betaRequest = betaRequest.WithContext(project.WithScope(betaRequest.Context(), project.Project{ID: "beta"}))
	betaResponse := httptest.NewRecorder()
	handler.ServeHTTP(betaResponse, betaRequest)
	if betaResponse.Code != http.StatusUnauthorized {
		t.Fatalf("alpha token on beta status = %d, want 401: %s", betaResponse.Code, betaResponse.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func signedProjectToken(t *testing.T, secret []byte, projectID string) string {
	t.Helper()
	encode := func(value []byte) string { return base64.RawURLEncoding.EncodeToString(value) }
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := json.Marshal(map[string]any{
		"sub":        "user-id",
		"role":       "authenticated",
		"aud":        "authenticated",
		"exp":        time.Now().Add(time.Hour).Unix(),
		"project_id": projectID,
	})
	if err != nil {
		t.Fatal(err)
	}
	unsigned := encode(header) + "." + encode(claims)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + encode(mac.Sum(nil))
}
