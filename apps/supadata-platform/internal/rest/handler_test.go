package rest

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestGETTableUsesPostgRESTPathAndReturnsRows(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer database.Close()
	query := `SELECT "id", "title" FROM "public"."todos" WHERE "id" = $1 LIMIT 10`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("42").WillReturnRows(
		sqlmock.NewRows([]string{"id", "title"}).AddRow(42, "Fix auth"),
	)

	handler := NewHandler(database, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodGet, "/rest/v1/todos?select=id,title&id=eq.42&limit=10", nil)
	request.Header.Set("apikey", "anon-key")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if response.Body.String() != `[{"id":42,"title":"Fix auth"}]`+"\n" {
		t.Fatalf("body = %q, want row JSON", response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestGETTableRequiresAPIKey(t *testing.T) {
	handler := NewHandler(nil, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodGet, "/rest/v1/todos", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}
