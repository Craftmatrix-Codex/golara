package auth

import (
	"context"
	"database/sql"
	"fmt"
)

// EnsurePostgresSchema reconciles the minimum PostgreSQL contract required by
// Golara Auth. It is intentionally idempotent so an existing Supabase-compatible
// auth schema can be upgraded without destructive migration steps.
func EnsurePostgresSchema(ctx context.Context, database *sql.DB, schema string) error {
	if database == nil {
		return fmt.Errorf("database connection is required")
	}
	if schema == "" {
		schema = defaultAuthSchema
	}
	if !validIdentifier(schema) {
		return fmt.Errorf("invalid auth schema")
	}

	quotedSchema := quoteIdentifier(schema)
	statements := []string{
		fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, quotedSchema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.users (
			id uuid PRIMARY KEY,
			instance_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000'::uuid,
			aud text NOT NULL DEFAULT 'authenticated',
			role text NOT NULL DEFAULT 'authenticated',
			email text,
			encrypted_password text,
			confirmed_at timestamptz,
			email_confirmed_at timestamptz,
			phone text,
			phone_confirmed_at timestamptz,
			raw_app_meta_data jsonb NOT NULL DEFAULT '{}'::jsonb,
			raw_user_meta_data jsonb NOT NULL DEFAULT '{}'::jsonb,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now(),
			deleted_at timestamptz
		)`, quotedSchema),
		fmt.Sprintf(`ALTER TABLE %s.users
			ADD COLUMN IF NOT EXISTS aud text NOT NULL DEFAULT 'authenticated',
			ADD COLUMN IF NOT EXISTS role text NOT NULL DEFAULT 'authenticated',
			ADD COLUMN IF NOT EXISTS raw_app_meta_data jsonb NOT NULL DEFAULT '{}'::jsonb,
			ADD COLUMN IF NOT EXISTS raw_user_meta_data jsonb NOT NULL DEFAULT '{}'::jsonb`, quotedSchema),
		fmt.Sprintf(`ALTER TABLE %s.users
			ADD COLUMN IF NOT EXISTS encrypted_password text`, quotedSchema),
		fmt.Sprintf(`ALTER TABLE %s.users
			ADD COLUMN IF NOT EXISTS deleted_at timestamptz`, quotedSchema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sessions (
			id uuid PRIMARY KEY,
			user_id uuid NOT NULL REFERENCES %s.users(id) ON DELETE CASCADE,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now(),
			not_after timestamptz,
			refreshed_at timestamptz
		)`, quotedSchema, quotedSchema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.refresh_tokens (
			id bigserial PRIMARY KEY,
			token text NOT NULL,
			user_id text NOT NULL,
			revoked boolean NOT NULL DEFAULT false,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now(),
			session_id uuid REFERENCES %s.sessions(id) ON DELETE CASCADE
		)`, quotedSchema, quotedSchema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS auth_users_email_unique
			ON %s.users (lower(email)) WHERE email IS NOT NULL AND deleted_at IS NULL`, quotedSchema),
	}

	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin auth schema reconciliation: %w", err)
	}
	defer transaction.Rollback()

	for _, statement := range statements {
		if _, err := transaction.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("reconcile auth schema: %w", err)
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit auth schema reconciliation: %w", err)
	}
	return nil
}
