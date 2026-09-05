package rest

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestRPCRouteExecutesNamedFunctionQuery(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "public"."add_todo"("title" := $1)`)).WithArgs("Fix auth").WillReturnRows(sqlmock.NewRows([]string{"result"}).AddRow("ok"))
	handler := NewHandler(database, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodPost, "/rest/v1/rpc/add_todo", strings.NewReader(`{"title":"Fix auth"}`))
	request.Header.Set("apikey", "anon-key")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != `[{"result":"ok"}]
` {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
