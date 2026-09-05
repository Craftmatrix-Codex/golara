package rest

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestPOSTTableInsertsObjectAndReturnsRepresentation(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer database.Close()
	query := `INSERT INTO "public"."todos" ("id", "title") VALUES ($1, $2) RETURNING *`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(float64(42), "Fix auth").WillReturnRows(
		sqlmock.NewRows([]string{"id", "title"}).AddRow(42, "Fix auth"),
	)

	handler := NewHandler(database, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodPost, "/rest/v1/todos", strings.NewReader(`{"id":42,"title":"Fix auth"}`))
	request.Header.Set("apikey", "anon-key")
	request.Header.Set("Prefer", "return=representation")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", response.Code, response.Body.String())
	}
	if response.Body.String() != `[{"id":42,"title":"Fix auth"}]`+"\n" {
		t.Fatalf("body = %q, want inserted representation", response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}
