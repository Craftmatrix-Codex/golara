package graphql

import (
	"database/sql/driver"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/database"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

func TestGraphQLUsesRequestScopedDatabase(t *testing.T) {
	primary, primaryMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer primary.Close()
	scoped, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer scoped.Close()
	mock.ExpectBegin()
	mock.ExpectExec(`SAVEPOINT graphql_schema_refresh`).WillReturnResult(driver.RowsAffected(0))
	mock.ExpectExec(`SELECT graphql\.increment_schema_version\(\)`).WillReturnResult(driver.RowsAffected(1))
	mock.ExpectExec(`RELEASE SAVEPOINT graphql_schema_refresh`).WillReturnResult(driver.RowsAffected(0))
	mock.ExpectQuery(`SELECT last_value FROM graphql\.seq_schema_version`).WillReturnRows(sqlmock.NewRows([]string{"last_value"}).AddRow(42))
	mock.ExpectExec(`SELECT\s+set_config\('role'`).WithArgs("anon", sqlmock.AnyArg(), "", "beta").WillReturnResult(driver.RowsAffected(1))
	mock.ExpectQuery(`SELECT graphql\.resolve`).WillReturnRows(sqlmock.NewRows([]string{"resolve"}).AddRow([]byte(`{"data":{"project":"beta"}}`)))
	mock.ExpectCommit()
	h := NewHandler(HandlerOptions{Database: primary, APIKeys: APIKeyConfig{Anon: "anon"}})
	r := httptest.NewRequest("POST", "/graphql/v1", strings.NewReader(`{"query":"{ viewer { id } }"}`))
	r.Header.Set("apikey", "anon")
	ctx := project.WithScope(r.Context(), project.Project{ID: "beta"})
	r = r.WithContext(database.WithConnection(ctx, scoped))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"project":"beta"`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := primaryMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
