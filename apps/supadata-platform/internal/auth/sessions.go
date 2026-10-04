package auth

import (
	"context"
	"fmt"
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
	transaction, err := r.databaseForContext(ctx).BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin revoke transaction: %w", err)
	}
	defer transaction.Rollback()
	quotedSchema := quoteIdentifier(r.schema)
	if _, err := transaction.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s.refresh_tokens
		SET revoked = true, updated_at = NOW()
		WHERE session_id = $1 AND revoked = false`, quotedSchema), sessionID); err != nil {
		return fmt.Errorf("revoke session refresh tokens: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s.sessions
		SET not_after = NOW(), updated_at = NOW()
		WHERE id = $1`, quotedSchema), sessionID); err != nil {
		return fmt.Errorf("expire auth session: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit revoke transaction: %w", err)
	}
	return nil
}
