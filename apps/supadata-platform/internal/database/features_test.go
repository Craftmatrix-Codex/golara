package database

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestEnsurePlatformFeaturesInstallsCronAndRealtimePublication(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT current_database\(\), current_setting\('cron.database_name', true\)`).
		WillReturnRows(sqlmock.NewRows([]string{"current_database", "cron_database"}).AddRow("supadata", "supadata"))
	mock.ExpectExec(`CREATE EXTENSION IF NOT EXISTS pg_cron`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`DO \$\$`).WillReturnResult(sqlmock.NewResult(0, 0))

	if err := EnsurePlatformFeatures(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsurePlatformFeaturesSkipsCronWhenConfiguredForAnotherDatabase(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT current_database\(\), current_setting\('cron.database_name', true\)`).
		WillReturnRows(sqlmock.NewRows([]string{"current_database", "cron_database"}).AddRow("supadata", "postgres"))
	mock.ExpectExec(`DO \$\$`).WillReturnResult(sqlmock.NewResult(0, 0))
	if err := EnsurePlatformFeatures(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
