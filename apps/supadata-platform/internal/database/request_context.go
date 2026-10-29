package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

// SetRequestContext installs transaction-local role and claims in one round trip.
// A pooled connection must never retain these settings after commit or rollback.
func SetRequestContext(ctx context.Context, tx *sql.Tx, claims jwt.Claims, fallbackRole string) error {
	role := claims.Role
	if role == "" {
		role = fallbackRole
	}
	if role != "anon" && role != "authenticated" && role != "service_role" {
		return errors.New("unsupported database role")
	}
	claims.Role = role
	projectID := claims.ProjectID
	if scope, ok := project.ScopeFromContext(ctx); ok {
		if projectID != "" && projectID != scope.ID {
			return errors.New("project token mismatch")
		}
		projectID = scope.ID
		claims.ProjectID = projectID
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `SELECT
		set_config('role', $1, true),
		set_config('request.jwt.claims', $2, true),
		set_config('request.jwt.claim.sub', $3, true),
		set_config('request.jwt.claim.role', $1, true),
		set_config('request.jwt.claim.project_id', $4, true)`, role, string(payload), claims.Subject, projectID)
	return err
}
