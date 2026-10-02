package graphql

import (
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHandlerResolvesGraphQLThroughPgGraphql(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.MatchExpectationsInOrder(false)
	mock.ExpectBegin()
	mock.ExpectExec(`SET LOCAL ROLE "anon"`).WillReturnResult(driver.RowsAffected(0))
	mock.ExpectQuery(`select set_config`).WillReturnRows(sqlmock.NewRows([]string{"set_config"}).AddRow(""))
	mock.ExpectQuery(`select set_config`).WillReturnRows(sqlmock.NewRows([]string{"set_config"}).AddRow(""))
	mock.ExpectQuery(`select set_config`).WillReturnRows(sqlmock.NewRows([]string{"set_config"}).AddRow(""))
	mock.ExpectQuery(`SELECT graphql\.resolve\(\$1, \$2::jsonb, \$3, \$4::jsonb\)`).
		WithArgs("{ viewer { id } }", `{"id":1}`, nil, `{"trace":true}`).
		WillReturnRows(sqlmock.NewRows([]string{"resolve"}).AddRow([]byte(`{"data":{"viewer":{"id":"1"}}}`)))
	mock.ExpectCommit()

	handler := NewHandler(HandlerOptions{Database: db, APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodPost, "/graphql/v1", strings.NewReader(`{"query":"{ viewer { id } }","variables":{"id":1},"extensions":{"trace":true}}`))
	request.Header.Set("apikey", "anon-key")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Logf("expectations = %v", mock.ExpectationsWereMet())
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["data"] == nil {
		t.Fatalf("response body = %s", response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHandlerRejectsMissingAPIKey(t *testing.T) {
	handler := NewHandler(HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodPost, "/graphql/v1", strings.NewReader(`{"query":"{ __typename }"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestHandlerRejectsMalformedRequest(t *testing.T) {
	handler := NewHandler(HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodPost, "/graphql/v1", strings.NewReader(`{"variables":{}}`))
	request.Header.Set("apikey", "anon-key")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
