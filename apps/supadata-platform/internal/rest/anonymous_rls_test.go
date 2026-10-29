package rest

import (
	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"net/http/httptest"
	"regexp"
	"testing"
)

func expectRoleTransaction(mock sqlmock.Sqlmock, role string) {
	mock.ExpectBegin()
	for _, key := range []string{"request.jwt.claims", "request.jwt.claim.sub", "request.jwt.claim.role"} {
		mock.ExpectQuery(regexp.QuoteMeta("select set_config($1, $2, true)")).WithArgs(key, sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"set_config"}).AddRow(""))
	}
	mock.ExpectExec(regexp.QuoteMeta(`SET LOCAL ROLE "` + role + `"`)).WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestAnonymousSDKRequestInstallsAnonRoleBeforeExecutingSQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	expectRoleTransaction(mock, "anon")
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "public"."todos"`)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectCommit()
	h := NewHandler(db, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon"}})
	r := httptest.NewRequest("GET", "/rest/v1/todos", nil)
	r.Header.Set("apikey", "anon")
	r.Header.Set("Authorization", "Bearer anon")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
