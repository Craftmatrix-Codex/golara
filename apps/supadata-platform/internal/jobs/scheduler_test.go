package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestEnsureSchemaCreatesPgCronCompatibleContracts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec("CREATE SCHEMA IF NOT EXISTS cron").WillReturnResult(sqlmock.NewResult(0, 0))
	if err := EnsureSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestScheduleIsDueSupportsIntervalsAndCronExpressions(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 10, 30, 0, time.UTC)
	tests := []struct {
		name     string
		schedule string
		last     time.Time
		want     bool
	}{
		{name: "seconds", schedule: "5 seconds", last: now.Add(-6 * time.Second), want: true},
		{name: "seconds not elapsed", schedule: "5 seconds", last: now.Add(-4 * time.Second), want: false},
		{name: "every minute", schedule: "* * * * *", last: now.Add(-time.Minute), want: true},
		{name: "step minute", schedule: "*/5 * * * *", last: now.Add(-time.Minute), want: true},
		{name: "fixed hour", schedule: "10 12 * * *", last: now.Add(-time.Hour), want: true},
		{name: "wrong minute", schedule: "11 12 * * *", last: now.Add(-time.Hour), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := scheduleIsDue(test.schedule, test.last, now); got != test.want {
				t.Fatalf("scheduleIsDue(%q)=%v want %v", test.schedule, got, test.want)
			}
		})
	}
}

func TestSchedulerExecutesAndRecordsDueJob(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 10, 4, 12, 10, 30, 0, time.UTC)
	mock.ExpectQuery("SELECT jobid, schedule, command, last_scheduled_at").
		WillReturnRows(sqlmock.NewRows([]string{"jobid", "schedule", "command", "last_scheduled_at"}).AddRow(1, "1 second", "insert into public.probe default values", now.Add(-2*time.Second)))
	mock.ExpectExec("UPDATE cron.job SET last_scheduled_at").WithArgs(int64(1), now, now.Add(-2*time.Second)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("insert into public.probe default values").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO cron.job_run_details").WillReturnResult(sqlmock.NewResult(1, 1))

	scheduler := NewScheduler(db, SchedulerOptions{Now: func() time.Time { return now }})
	if err := scheduler.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
