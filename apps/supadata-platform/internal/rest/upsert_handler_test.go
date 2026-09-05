package rest

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestPOSTWithOnConflictPerformsUpsert(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer database.Close()
	query := `INSERT INTO "public"."todos" ("id", "title") VALUES ($1, $2) ON CONFLICT ("id") DO UPDATE SET "title" = EXCLUDED."title" RETURNING *`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("42", "Fixed").WillReturnRows(
		sqlmock.NewRows([]string{"id", "title"}).AddRow(42, "Fixed"),
	)

	handler := NewHandler(database, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodPost, "/rest/v1/todos?on_conflict=id", strings.NewReader(`{"id":"42","title":"Fixed"}`))
	request.Header.Set("apikey", "anon-key")
	request.Header.Set("Prefer", "return=representation,resolution=merge-duplicates")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || response.Body.String() != `[{"id":42,"title":"Fixed"}]`+"\n" {
		t.Fatalf("status=%d body=%q, want upserted representation", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}
