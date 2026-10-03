package auth

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestEnsurePostgresSchemaReconcilesAuthRuntimeTables(t *testing.T) {
	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	mock.ExpectBegin()
	for _, fragment := range []string{
		`CREATE SCHEMA IF NOT EXISTS "auth"`,
		`CREATE TABLE IF NOT EXISTS "auth".users`,
		`ALTER TABLE "auth".users ADD COLUMN IF NOT EXISTS aud`,
		`ALTER TABLE "auth".users ADD COLUMN IF NOT EXISTS encrypted_password`,
		`ALTER TABLE "auth".users ADD COLUMN IF NOT EXISTS deleted_at`,
		`CREATE TABLE IF NOT EXISTS "auth".sessions`,
		`CREATE TABLE IF NOT EXISTS "auth".refresh_tokens`,
		`CREATE UNIQUE INDEX IF NOT EXISTS auth_users_email_unique`,
	} {
		mock.ExpectExec(regexp.QuoteMeta(fragment) + `(?s:.*)`).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectCommit()

	if err := EnsurePostgresSchema(context.Background(), database, "auth"); err != nil {
		t.Fatalf("EnsurePostgresSchema() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsurePostgresSchemaRejectsInvalidIdentifier(t *testing.T) {
	if err := EnsurePostgresSchema(context.Background(), &sql.DB{}, `auth; DROP SCHEMA public`); err == nil {
		t.Fatal("EnsurePostgresSchema() accepted an unsafe schema identifier")
	}
}
