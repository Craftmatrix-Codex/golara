package auth

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestDefaultAdminDeleteRemovesUserRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo, err := NewPostgresRepository(db, "auth")
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec(`DELETE FROM "auth"\.users WHERE id = \$1`).WithArgs("user-1").WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repo.DeleteUser(context.Background(), "user-1"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
