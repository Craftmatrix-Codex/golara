package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

func (r *PostgresRepository) IsSessionActive(ctx context.Context, sessionID string) (bool, error) {
	quotedSchema := quoteIdentifier(r.schema)
	var isActive bool
	err := r.databaseForContext(ctx).QueryRowContext(ctx, fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM %s.sessions
			WHERE id = $1 AND not_after > NOW()
		)`, quotedSchema), sessionID).Scan(&isActive)
	if err != nil {
		return false, fmt.Errorf("validate auth session: %w", err)
	}
	return isActive, nil
}

func (r *PostgresRepository) RevokeSession(ctx context.Context, sessionID string) error {
	projectID := "default"
	if scope, ok := project.ScopeFromContext(ctx); ok && scope.ID != "" {
		projectID = scope.ID
	}
	transaction, err := r.databaseForContext(ctx).BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin revoke transaction: %w", err)
	}
	defer transaction.Rollback()
	schema := quoteIdentifier(r.schema)
	// Serialize local and global logout for the same user, then lock refresh
	// tokens before sessions, matching the order used for token rotation.
	var userID string
	lock := fmt.Sprintf(`SELECT u.id FROM %s.users u JOIN %s.sessions s ON s.user_id = u.id WHERE s.id = $1 AND s.not_after > NOW() AND (s.golara_project_id = $2 OR (s.golara_project_id IS NULL AND $2 = 'default')) FOR UPDATE OF u`, schema, schema)
	if err := transaction.QueryRowContext(ctx, lock, sessionID, projectID).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if projectID != "default" {
				var legacy bool
				query := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s.sessions WHERE id=$1 AND golara_project_id IS NULL AND not_after > NOW())`, schema)
				if err := transaction.QueryRowContext(ctx, query, sessionID).Scan(&legacy); err != nil {
					return fmt.Errorf("check legacy session: %w", err)
				}
				if legacy {
					return ErrUnscopedLegacySessions
				}
			}
			return ErrInvalidCredentials
		}
		return fmt.Errorf("lock session user: %w", err)
	}
	// Acquire refresh-token locks before session locks, matching refresh rotation.
	if _, err := transaction.ExecContext(ctx, fmt.Sprintf(`UPDATE %s.refresh_tokens SET revoked = true, updated_at = NOW() WHERE session_id = $1 AND revoked = false AND EXISTS (SELECT 1 FROM %s.sessions s WHERE s.id = $1 AND s.not_after > NOW() AND (s.golara_project_id = $2 OR (s.golara_project_id IS NULL AND $2 = 'default')))`, schema, schema), sessionID, projectID); err != nil {
		return fmt.Errorf("revoke session refresh tokens: %w", err)
	}
	result, err := transaction.ExecContext(ctx, fmt.Sprintf(`UPDATE %s.sessions SET not_after = NOW(), updated_at = NOW() WHERE id = $1 AND not_after > NOW() AND (golara_project_id = $2 OR (golara_project_id IS NULL AND $2 = 'default'))`, schema), sessionID, projectID)
	if err != nil {
		return fmt.Errorf("expire auth session: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check auth session: %w", err)
	}
	if count != 1 {
		return ErrInvalidCredentials
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit revoke transaction: %w", err)
	}
	return nil
}

var ErrUnscopedLegacySessions = errors.New("other active sessions have no project scope")

// RevokeUserSessions rejects ambiguous legacy sessions rather than revoking
// sessions that might belong to another project on a shared database.
func (r *PostgresRepository) RevokeUserSessions(ctx context.Context, userID, currentSessionID string, others bool) error {
	projectID := "default"
	if scope, ok := project.ScopeFromContext(ctx); ok && scope.ID != "" {
		projectID = scope.ID
	}
	db := r.databaseForContext(ctx)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin scoped session revocation: %w", err)
	}
	defer tx.Rollback()
	schema := quoteIdentifier(r.schema)
	// Lock the user row so concurrent session inserts wait for this transaction.
	var locked string
	if err := tx.QueryRowContext(ctx, fmt.Sprintf(`SELECT id FROM %s.users WHERE id = $1 FOR UPDATE`, schema), userID).Scan(&locked); err != nil {
		return fmt.Errorf("lock session user: %w", err)
	}
	var currentActive bool
	query := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s.sessions WHERE id = $1 AND user_id = $2 AND not_after > NOW() AND (golara_project_id = $3 OR (golara_project_id IS NULL AND $3 = 'default')))`, schema)
	if err := tx.QueryRowContext(ctx, query, currentSessionID, userID, projectID).Scan(&currentActive); err != nil {
		return fmt.Errorf("validate current session: %w", err)
	}
	if !currentActive {
		if projectID != "default" {
			var legacy bool
			query := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s.sessions WHERE id=$1 AND user_id=$2 AND golara_project_id IS NULL AND not_after > NOW())`, schema)
			if err := tx.QueryRowContext(ctx, query, currentSessionID, userID).Scan(&legacy); err != nil {
				return fmt.Errorf("check legacy session: %w", err)
			}
			if legacy {
				return ErrUnscopedLegacySessions
			}
		}
		return ErrInvalidCredentials
	}
	var ambiguous bool
	query = fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s.sessions s WHERE s.user_id = $1 AND s.id <> $2 AND s.golara_project_id IS NULL AND (s.not_after > NOW() OR EXISTS (SELECT 1 FROM %s.refresh_tokens t WHERE t.session_id = s.id AND t.revoked = false)))`, schema, schema)
	if err := tx.QueryRowContext(ctx, query, userID, currentSessionID).Scan(&ambiguous); err != nil {
		return fmt.Errorf("check legacy sessions: %w", err)
	}
	if ambiguous {
		return ErrUnscopedLegacySessions
	}
	exclude := ""
	if others {
		exclude = currentSessionID
	}
	query = fmt.Sprintf(`UPDATE %s.refresh_tokens t SET revoked = true, updated_at = NOW() FROM %s.sessions s WHERE t.session_id = s.id AND s.user_id = $1 AND (s.golara_project_id = $2 OR s.id = $3) AND ($4 = '' OR s.id::text <> $4) AND t.revoked = false`, schema, schema)
	if _, err := tx.ExecContext(ctx, query, userID, projectID, currentSessionID, exclude); err != nil {
		return fmt.Errorf("revoke scoped refresh tokens: %w", err)
	}
	query = fmt.Sprintf(`UPDATE %s.sessions SET not_after = NOW(), updated_at = NOW() WHERE user_id = $1 AND (golara_project_id = $2 OR id = $3) AND ($4 = '' OR id::text <> $4)`, schema)
	if _, err := tx.ExecContext(ctx, query, userID, projectID, currentSessionID, exclude); err != nil {
		return fmt.Errorf("expire scoped sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit scoped session revocation: %w", err)
	}
	return nil
}
