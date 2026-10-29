package rest

import (
	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestSDKSingleReturnsAnObjectAndRejectsZeroOrMultipleRows(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			expectRoleTransaction(mock, "anon")
			rows := sqlmock.NewRows([]string{"id"})
			for i := 0; i < count; i++ {
				rows.AddRow(i + 1)
			}
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "public"."todos"`)).WillReturnRows(rows)
			if count == 1 {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			r := httptest.NewRequest("GET", "/rest/v1/todos", nil)
			r.Header.Set("apikey", "anon")
			r.Header.Set("Accept", "application/vnd.pgrst.object+json")
			w := httptest.NewRecorder()
			NewHandler(db, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon"}}).ServeHTTP(w, r)
			if count == 1 {
				if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"id":1}` {
					t.Fatalf("response=%d %s", w.Code, w.Body.String())
				}
			} else if w.Code != 406 || !strings.Contains(w.Body.String(), "PGRST116") {
				t.Fatalf("response=%d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestSDKHeadExactCountReturnsNoBodyAndFilteredTotal(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	expectRoleTransaction(mock, "anon")
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) AS "count" FROM (SELECT * FROM "public"."todos" WHERE "title" = $1) AS "golara_count"`)).WithArgs("sdk").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3)))
	mock.ExpectCommit()
	r := httptest.NewRequest("HEAD", "/rest/v1/todos?title=eq.sdk&limit=1", nil)
	r.Header.Set("apikey", "anon")
	r.Header.Set("Prefer", "count=exact")
	w := httptest.NewRecorder()
	NewHandler(db, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon"}}).ServeHTTP(w, r)
	if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Range") != "0-0/3" {
		t.Fatalf("response=%d range=%s body=%s", w.Code, w.Header().Get("Content-Range"), w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSDKSingletonMutationRollsBackWhenMoreThanOneRowMatches(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	expectRoleTransaction(mock, "anon")
	mock.ExpectQuery(regexp.QuoteMeta(`UPDATE "public"."todos" SET "title" = $1 WHERE "id" >= $2 RETURNING *`)).WithArgs("new", "1").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
	mock.ExpectRollback()
	r := httptest.NewRequest("PATCH", "/rest/v1/todos?id=gte.1", strings.NewReader(`{"title":"new"}`))
	r.Header.Set("apikey", "anon")
	r.Header.Set("Prefer", "return=representation")
	r.Header.Set("Accept", "application/vnd.pgrst.object+json")
	w := httptest.NewRecorder()
	NewHandler(db, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon"}}).ServeHTTP(w, r)
	if w.Code != 406 || !strings.Contains(w.Body.String(), "PGRST116") {
		t.Fatalf("response=%d %s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
