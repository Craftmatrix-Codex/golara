package rest

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestPATCHTableUpdatesOnlyFilteredRows(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer database.Close()
	query := `UPDATE "public"."todos" SET "title" = $1 WHERE "id" = $2 RETURNING *`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("Fixed", "42").WillReturnRows(
		sqlmock.NewRows([]string{"id", "title"}).AddRow(42, "Fixed"),
	)

	handler := NewHandler(database, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodPatch, "/rest/v1/todos?id=eq.42", strings.NewReader(`{"title":"Fixed"}`))
	request.Header.Set("apikey", "anon-key")
	request.Header.Set("Prefer", "return=representation")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != `[{"id":42,"title":"Fixed"}]`+"\n" {
		t.Fatalf("status=%d body=%q, want updated representation", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}
