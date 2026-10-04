package database

import (
	"context"
	"database/sql"
	"fmt"
)

func EnsurePlatformFeatures(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	var currentDatabase string
	var cronDatabase sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT current_database(), current_setting('cron.database_name', true)`).Scan(&currentDatabase, &cronDatabase); err != nil {
		return fmt.Errorf("inspect pg_cron database: %w", err)
	}
	if cronDatabase.Valid && cronDatabase.String == currentDatabase {
		if _, err := db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS pg_cron`); err != nil {
			return fmt.Errorf("install pg_cron: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, `DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'supabase_realtime') THEN
    CREATE PUBLICATION supabase_realtime;
  END IF;
END
$$`); err != nil {
		return fmt.Errorf("create realtime publication: %w", err)
	}
	return nil
}
