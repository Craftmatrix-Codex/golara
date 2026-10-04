package realtime

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/database"
)

var postgresIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

const changeQueueSchemaSQL = `
CREATE SCHEMA IF NOT EXISTS supadata_realtime;
CREATE TABLE IF NOT EXISTS supadata_realtime.events (
  id bigserial PRIMARY KEY,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  event_type text NOT NULL,
  record jsonb,
  old_record jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE OR REPLACE FUNCTION supadata_realtime.capture_change()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = ''
AS $$
BEGIN
  INSERT INTO supadata_realtime.events (schema_name, table_name, event_type, record, old_record)
  VALUES (
    TG_TABLE_SCHEMA,
    TG_TABLE_NAME,
    TG_OP,
    CASE WHEN TG_OP = 'DELETE' THEN NULL ELSE to_jsonb(NEW) END,
    CASE WHEN TG_OP = 'INSERT' THEN NULL ELSE to_jsonb(OLD) END
  );
  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END;
$$;
DELETE FROM supadata_realtime.events WHERE created_at < now() - interval '1 day';`

const pollChangesSQL = `
SELECT id, schema_name, table_name, event_type, record, old_record
FROM supadata_realtime.events
WHERE id > $1 AND schema_name = $2 AND table_name = $3 AND ($4 = '*' OR event_type = $4)
ORDER BY id
LIMIT 100`

type PostgresChangeSourceOptions struct {
	PollInterval time.Duration
}

type PostgresChangeSource struct {
	pollInterval time.Duration
}

func NewPostgresChangeSource(options PostgresChangeSourceOptions) *PostgresChangeSource {
	interval := options.PollInterval
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	return &PostgresChangeSource{pollInterval: interval}
}

func (s *PostgresChangeSource) Subscribe(ctx context.Context, _ string, subscription ChangeSubscription, emit func(ChangeEvent)) (func(), error) {
	if err := validateChangeSubscription(subscription); err != nil {
		return nil, err
	}
	db, ok := database.ConnectionFromContext(ctx)
	if !ok {
		return nil, errors.New("project database unavailable")
	}
	if _, err := db.ExecContext(ctx, changeQueueSchemaSQL); err != nil {
		return nil, fmt.Errorf("prepare realtime change queue: %w", err)
	}
	var cursor int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), 0) FROM supadata_realtime.events WHERE schema_name = $1 AND table_name = $2`, subscription.Schema, subscription.Table).Scan(&cursor); err != nil {
		return nil, fmt.Errorf("read realtime cursor: %w", err)
	}
	if _, err := db.ExecContext(ctx, changeTriggerSQL(subscription.Schema, subscription.Table)); err != nil {
		return nil, fmt.Errorf("install realtime trigger: %w", err)
	}
	subscriptionContext, cancel := context.WithCancel(ctx)
	go s.poll(subscriptionContext, db, cursor, subscription, emit)
	return cancel, nil
}

func (s *PostgresChangeSource) poll(ctx context.Context, db *sql.DB, cursor int64, subscription ChangeSubscription, emit func(ChangeEvent)) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			next, err := s.pollOnce(ctx, db, cursor, subscription, emit)
			if err == nil {
				cursor = next
			}
		}
	}
}

func (s *PostgresChangeSource) pollOnce(ctx context.Context, db *sql.DB, cursor int64, subscription ChangeSubscription, emit func(ChangeEvent)) (int64, error) {
	rows, err := db.QueryContext(ctx, pollChangesSQL, cursor, subscription.Schema, subscription.Table, subscription.Event)
	if err != nil {
		return cursor, err
	}
	defer rows.Close()
	next := cursor
	for rows.Next() {
		var id int64
		var event ChangeEvent
		var record, oldRecord []byte
		if err := rows.Scan(&id, &event.Schema, &event.Table, &event.Event, &record, &oldRecord); err != nil {
			return cursor, err
		}
		if len(record) > 0 {
			if err := json.Unmarshal(record, &event.Record); err != nil {
				return cursor, err
			}
		}
		if len(oldRecord) > 0 {
			if err := json.Unmarshal(oldRecord, &event.OldRecord); err != nil {
				return cursor, err
			}
		}
		emit(event)
		next = id
	}
	return next, rows.Err()
}

func validateChangeSubscription(subscription ChangeSubscription) error {
	if !postgresIdentifierPattern.MatchString(subscription.Schema) || !postgresIdentifierPattern.MatchString(subscription.Table) {
		return errors.New("invalid realtime relation")
	}
	switch strings.ToUpper(subscription.Event) {
	case "*", "INSERT", "UPDATE", "DELETE":
		return nil
	default:
		return errors.New("invalid realtime event")
	}
}

func changeTriggerSQL(schema, table string) string {
	digest := sha256.Sum256([]byte(schema + "." + table))
	trigger := "supadata_realtime_" + hex.EncodeToString(digest[:6])
	qualified := quoteIdentifier(schema) + "." + quoteIdentifier(table)
	return fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON %s;
CREATE TRIGGER %s AFTER INSERT OR UPDATE OR DELETE ON %s
FOR EACH ROW EXECUTE FUNCTION supadata_realtime.capture_change()`, quoteIdentifier(trigger), qualified, quoteIdentifier(trigger), qualified)
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
