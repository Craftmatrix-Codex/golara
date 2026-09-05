package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

const defaultAuthSchema = "auth"

type PostgresRepository struct {
	db     *sql.DB
	schema string
}

func NewPostgresRepository(db *sql.DB, schema string) (*PostgresRepository, error) {
	if db == nil {
		return nil, errors.New("database connection is required")
	}
	if schema == "" {
		schema = defaultAuthSchema
	}
	if !validIdentifier(schema) {
		return nil, errors.New("invalid auth schema")
	}
	return &PostgresRepository{db: db, schema: schema}, nil
}

func (r *PostgresRepository) CreateUser(ctx context.Context, email, passwordHash string, metadata map[string]any, confirmed bool) (User, error) {
	metadataJSON, err := json.Marshal(nonNilMetadata(metadata))
	if err != nil {
		return User{}, fmt.Errorf("encode user metadata: %w", err)
	}
	confirmedAt := any(nil)
	if confirmed {
		confirmedAt = time.Now().UTC()
	}
	query := fmt.Sprintf(`
		INSERT INTO %s.users
			(id, aud, role, email, encrypted_password, confirmed_at, raw_app_meta_data, raw_user_meta_data, created_at, updated_at)
		VALUES ($1, 'authenticated', 'authenticated', $2, $3, $4, '{}'::jsonb, $5::jsonb, NOW(), NOW())
		RETURNING id, email, role, raw_app_meta_data, raw_user_meta_data, created_at, updated_at, confirmed_at`, quoteIdentifier(r.schema))
	id, err := newUUID()
	if err != nil {
		return User{}, err
	}
	return r.scanUser(r.db.QueryRowContext(ctx, query, id, email, passwordHash, confirmedAt, metadataJSON), "create user")
}

func (r *PostgresRepository) FindUserByEmail(ctx context.Context, email string) (User, string, error) {
	query := fmt.Sprintf(`
		SELECT id, email, role, encrypted_password, raw_app_meta_data, raw_user_meta_data, created_at, updated_at, confirmed_at
		FROM %s.users
		WHERE lower(email) = lower($1) AND deleted_at IS NULL
		LIMIT 1`, quoteIdentifier(r.schema))
	var user User
	var passwordHash string
	var appMetadataJSON, userMetadataJSON []byte
	var confirmedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Role, &passwordHash, &appMetadataJSON, &userMetadataJSON,
		&user.CreatedAt, &user.UpdatedAt, &confirmedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, "", ErrUserNotFound
		}
		return User{}, "", fmt.Errorf("find user: %w", err)
	}
	if err := decodeMetadata(appMetadataJSON, &user.AppMetadata); err != nil {
		return User{}, "", err
	}
	if err := decodeMetadata(userMetadataJSON, &user.UserMetadata); err != nil {
		return User{}, "", err
	}
	if confirmedAt.Valid {
		user.EmailConfirmedAt = &confirmedAt.Time
	}
	return user, passwordHash, nil
}

func (r *PostgresRepository) FindUserByID(ctx context.Context, id string) (User, error) {
	query := fmt.Sprintf(`
		SELECT id, email, role, raw_app_meta_data, raw_user_meta_data, created_at, updated_at, confirmed_at
		FROM %s.users
		WHERE id = $1 AND deleted_at IS NULL
		LIMIT 1`, quoteIdentifier(r.schema))
	return r.scanUser(r.db.QueryRowContext(ctx, query, id), "find user")
}

func (r *PostgresRepository) CreateSession(ctx context.Context, userID, refreshTokenHash string, expiresAt time.Time) (Session, error) {
	sessionID, err := newUUID()
	if err != nil {
		return Session{}, err
	}
	transaction, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, fmt.Errorf("begin session transaction: %w", err)
	}
	defer transaction.Rollback()
	quotedSchema := quoteIdentifier(r.schema)
	if _, err := transaction.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s.sessions (id, user_id, created_at, updated_at, not_after)
		VALUES ($1, $2, NOW(), NOW(), $3)`, quotedSchema), sessionID, userID, expiresAt); err != nil {
		return Session{}, fmt.Errorf("create auth session: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s.refresh_tokens (token, user_id, revoked, created_at, updated_at, session_id)
		VALUES ($1, $2, false, NOW(), NOW(), $3)`, quotedSchema), refreshTokenHash, userID, sessionID); err != nil {
		return Session{}, fmt.Errorf("create refresh token: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return Session{}, fmt.Errorf("commit auth session: %w", err)
	}
	return Session{ID: sessionID, UserID: userID, RefreshTokenHash: refreshTokenHash, ExpiresAt: expiresAt}, nil
}

func (r *PostgresRepository) scanUser(row *sql.Row, operation string) (User, error) {
	var user User
	var appMetadataJSON, userMetadataJSON []byte
	var confirmedAt sql.NullTime
	if err := row.Scan(&user.ID, &user.Email, &user.Role, &appMetadataJSON, &userMetadataJSON, &user.CreatedAt, &user.UpdatedAt, &confirmedAt); err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrUserAlreadyRegistered
		}
		return User{}, fmt.Errorf("%s: %w", operation, err)
	}
	if err := decodeMetadata(appMetadataJSON, &user.AppMetadata); err != nil {
		return User{}, err
	}
	if err := decodeMetadata(userMetadataJSON, &user.UserMetadata); err != nil {
		return User{}, err
	}
	if confirmedAt.Valid {
		user.EmailConfirmedAt = &confirmedAt.Time
	}
	return user, nil
}

func nonNilMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	return metadata
}

func decodeMetadata(data []byte, destination *map[string]any) error {
	if len(data) == 0 {
		*destination = map[string]any{}
		return nil
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("decode user metadata: %w", err)
	}
	if *destination == nil {
		*destination = map[string]any{}
	}
	return nil
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		isLetter := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
		isDigit := character >= '0' && character <= '9'
		if !isLetter && !isDigit && character != '_' {
			return false
		}
		if index == 0 && !isLetter && character != '_' {
			return false
		}
	}
	return true
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func newUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate user id: %w", err)
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := hex.EncodeToString(value)
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}

func isUniqueViolation(err error) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) && pgError.Code == "23505"
}
