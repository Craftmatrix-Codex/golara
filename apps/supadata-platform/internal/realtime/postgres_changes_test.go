package realtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/database"
)

func TestPostgresChangeSourcePreparesValidatedSubscription(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec("CREATE SCHEMA IF NOT EXISTS supadata_realtime").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(id\\), 0\\)").WithArgs("public", "messages").WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(0))
	mock.ExpectExec("DROP TRIGGER.*CREATE TRIGGER").WillReturnResult(sqlmock.NewResult(0, 0))

	ctx, cancel := context.WithCancel(database.WithConnection(context.Background(), db))
	source := NewPostgresChangeSource(PostgresChangeSourceOptions{PollInterval: time.Hour})
	unsubscribe, err := source.Subscribe(ctx, "default", ChangeSubscription{Event: "INSERT", Schema: "public", Table: "messages"}, func(ChangeEvent) {})
	if err != nil {
		t.Fatal(err)
	}
	unsubscribe()
	cancel()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresChangeSourceQuotesValidatedIdentifiers(t *testing.T) {
	query := changeTriggerSQL("public", "messages")
	if !strings.Contains(query, "ON \"public\".\"messages\"") || strings.Contains(query, "\\\"public\\\"") {
		t.Fatalf("unexpected trigger SQL quoting: %s", query)
	}
}

func TestPostgresChangeSourceRejectsUnsafeIdentifiers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := database.WithConnection(context.Background(), db)
	source := NewPostgresChangeSource(PostgresChangeSourceOptions{})
	if _, err := source.Subscribe(ctx, "default", ChangeSubscription{Event: "*", Schema: "public", Table: `messages; DROP TABLE users`}, func(ChangeEvent) {}); err == nil {
		t.Fatal("expected unsafe table identifier to be rejected")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresChangeSourcePollsChangePayload(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT id, schema_name, table_name, event_type, record, old_record").
		WithArgs(int64(4), "public", "messages", "INSERT").
		WillReturnRows(sqlmock.NewRows([]string{"id", "schema_name", "table_name", "event_type", "record", "old_record"}).
			AddRow(5, "public", "messages", "INSERT", []byte(`{"id":7,"body":"hello"}`), nil))

	source := NewPostgresChangeSource(PostgresChangeSourceOptions{})
	var received ChangeEvent
	cursor, err := source.pollOnce(context.Background(), db, 4, ChangeSubscription{Event: "INSERT", Schema: "public", Table: "messages"}, func(event ChangeEvent) {
		received = event
	})
	if err != nil {
		t.Fatal(err)
	}
	if cursor != 5 || received.Event != "INSERT" || received.Record["body"] != "hello" {
		t.Fatalf("unexpected cursor/event: %d %#v", cursor, received)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
