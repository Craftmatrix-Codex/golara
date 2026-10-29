package auth

import (
	"context"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
	"testing"
)

func TestLegacyUserTokenIsRestrictedToDefaultProject(t *testing.T) {
	for _, tc := range []struct {
		tokenProject, target string
		allowed              bool
	}{
		{"", "default", true}, {"", "alpha", false}, {"alpha", "alpha", true},
		{"alpha", "beta", false}, {"default", "alpha", false},
	} {
		t.Run(tc.tokenProject+"/"+tc.target, func(t *testing.T) {
			ctx := project.WithScope(context.Background(), project.Project{ID: tc.target})
			err := validateProjectClaims(ctx, jwt.Claims{ProjectID: tc.tokenProject})
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v error=%v", tc.allowed, err)
			}
		})
	}
}
