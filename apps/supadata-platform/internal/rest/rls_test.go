package rest

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
)

func TestGETWithJWTInstallsRequestClaimsInTransaction(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer database.Close()
	accessToken, err := jwt.SignHS256(jwt.Claims{Subject: "user-id", Role: "authenticated", ExpiresAt: 1_900_000_000}, []byte("jwt-secret"))
	if err != nil {
		t.Fatalf("SignHS256() error = %v", err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("select set_config($1, $2, true)")).WithArgs("request.jwt.claims", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"set_config"}).AddRow("claims"))
	mock.ExpectQuery(regexp.QuoteMeta("select set_config($1, $2, true)")).WithArgs("request.jwt.claim.sub", "user-id").WillReturnRows(sqlmock.NewRows([]string{"set_config"}).AddRow("user-id"))
	mock.ExpectQuery(regexp.QuoteMeta("select set_config($1, $2, true)")).WithArgs("request.jwt.claim.role", "authenticated").WillReturnRows(sqlmock.NewRows([]string{"set_config"}).AddRow("authenticated"))
	mock.ExpectExec(regexp.QuoteMeta(`SET LOCAL ROLE "authenticated"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "public"."todos"`)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	handler := NewHandler(database, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}, JWTSecret: []byte("jwt-secret")})
	request := httptest.NewRequest(http.MethodGet, "/rest/v1/todos", nil)
	request.Header.Set("apikey", "anon-key")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s; SQL expectations: %v", response.Code, response.Body.String(), mock.ExpectationsWereMet())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}
