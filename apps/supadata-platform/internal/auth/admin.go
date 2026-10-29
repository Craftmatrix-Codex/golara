package auth

import (
	"context"
	"database/sql"
	"fmt"
)

func (r *PostgresRepository) DeleteUser(ctx context.Context, userID string) error {
	return r.DeleteUserWithOptions(ctx, userID, false)
}

func (r *PostgresRepository) DeleteUserWithOptions(ctx context.Context, userID string, soft bool) error {
	query := fmt.Sprintf(`DELETE FROM %s.users WHERE id = $1`, quoteIdentifier(r.schema))
	if soft {
		query = fmt.Sprintf(`UPDATE %s.users SET deleted_at = NOW(), updated_at = NOW()
   WHERE id = $1 AND deleted_at IS NULL`, quoteIdentifier(r.schema))
	}
	result, err := r.databaseForContext(ctx).ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("delete auth user: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted auth user: %w", err)
	}
	if rowsAffected != 1 {
		return ErrUserNotFound
	}
	return nil
}

func (r *PostgresRepository) ListUsers(ctx context.Context, page, perPage int) ([]User, int, error) {
	if page < 1 || perPage < 1 || perPage > 1000 {
		return nil, 0, fmt.Errorf("invalid pagination")
	}
	quotedSchema := quoteIdentifier(r.schema)
	var total int
	if err := r.databaseForContext(ctx).QueryRowContext(ctx, fmt.Sprintf(`
		SELECT count(*) FROM %s.users WHERE deleted_at IS NULL`, quotedSchema)).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count auth users: %w", err)
	}
	// Keep the offset calculation bounded by the validated page/per-page contract.
	offset := (page - 1) * perPage
	rows, err := r.databaseForContext(ctx).QueryContext(ctx, fmt.Sprintf(`
		SELECT id, email, role, raw_app_meta_data, raw_user_meta_data, created_at, updated_at, confirmed_at
		FROM %s.users
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2`, quotedSchema), perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list auth users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0, perPage)
	for rows.Next() {
		var user User
		var appMetadataJSON, userMetadataJSON []byte
		var confirmedAt sql.NullTime
		if err := rows.Scan(&user.ID, &user.Email, &user.Role, &appMetadataJSON, &userMetadataJSON, &user.CreatedAt, &user.UpdatedAt, &confirmedAt); err != nil {
			return nil, 0, fmt.Errorf("scan auth user: %w", err)
		}
		if err := decodeMetadata(appMetadataJSON, &user.AppMetadata); err != nil {
			return nil, 0, err
		}
		if err := decodeMetadata(userMetadataJSON, &user.UserMetadata); err != nil {
			return nil, 0, err
		}
		if confirmedAt.Valid {
			user.EmailConfirmedAt = &confirmedAt.Time
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate auth users: %w", err)
	}
	return users, total, nil
}
