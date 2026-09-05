package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (r *PostgresRepository) RefreshSession(ctx context.Context, oldHash, newHash string, expiresAt time.Time) (User, Session, error) {
	transaction, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, Session{}, fmt.Errorf("begin refresh transaction: %w", err)
	}
	defer transaction.Rollback()
	quotedSchema := quoteIdentifier(r.schema)

	var userID, sessionID string
	if err := transaction.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT user_id, session_id
		FROM %s.refresh_tokens
		WHERE token = $1 AND revoked = false
		ORDER BY id DESC
		LIMIT 1
		FOR UPDATE`, quotedSchema), oldHash).Scan(&userID, &sessionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, Session{}, ErrInvalidRefreshToken
		}
		return User{}, Session{}, fmt.Errorf("find refresh token: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s.refresh_tokens SET revoked = true, updated_at = NOW()
		WHERE token = $1 AND revoked = false`, quotedSchema), oldHash); err != nil {
		return User{}, Session{}, fmt.Errorf("revoke refresh token: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s.refresh_tokens (token, user_id, revoked, created_at, updated_at, session_id)
		VALUES ($1, $2, false, NOW(), NOW(), $3)`, quotedSchema), newHash, userID, sessionID); err != nil {
		return User{}, Session{}, fmt.Errorf("create rotated refresh token: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s.sessions SET updated_at = NOW(), refreshed_at = NOW(), not_after = $2
		WHERE id = $1`, quotedSchema), sessionID, expiresAt); err != nil {
		return User{}, Session{}, fmt.Errorf("update auth session: %w", err)
	}

	var user User
	var appMetadataJSON, userMetadataJSON []byte
	var confirmedAt sql.NullTime
	if err := transaction.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id, email, role, raw_app_meta_data, raw_user_meta_data, created_at, updated_at, confirmed_at
		FROM %s.users WHERE id = $1 AND deleted_at IS NULL`, quotedSchema), userID).Scan(
		&user.ID, &user.Email, &user.Role, &appMetadataJSON, &userMetadataJSON,
		&user.CreatedAt, &user.UpdatedAt, &confirmedAt,
	); err != nil {
		return User{}, Session{}, fmt.Errorf("find refresh user: %w", err)
	}
	if err := decodeMetadata(appMetadataJSON, &user.AppMetadata); err != nil {
		return User{}, Session{}, err
	}
	if err := decodeMetadata(userMetadataJSON, &user.UserMetadata); err != nil {
		return User{}, Session{}, err
	}
	if confirmedAt.Valid {
		user.EmailConfirmedAt = &confirmedAt.Time
	}
	if err := transaction.Commit(); err != nil {
		return User{}, Session{}, fmt.Errorf("commit refresh transaction: %w", err)
	}
	return user, Session{ID: sessionID, UserID: userID, RefreshTokenHash: newHash, ExpiresAt: expiresAt}, nil
}
