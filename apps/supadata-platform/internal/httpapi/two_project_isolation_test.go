package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/auth"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/database"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/rest"
)

type scopedAuthRecorder struct {
	seen []string
}

func (r *scopedAuthRecorder) SignUp(ctx context.Context, _, _ string, _ map[string]any) (auth.SessionResponse, error) {
	scope, ok := project.ScopeFromContext(ctx)
	if !ok {
		return auth.SessionResponse{}, project.ErrNotFound
	}
	r.seen = append(r.seen, scope.ID)
	return auth.SessionResponse{User: auth.User{ID: scope.ID + "-user"}}, nil
}

func (*scopedAuthRecorder) SignIn(context.Context, string, string) (auth.SessionResponse, error) {
	return auth.SessionResponse{}, nil
}

func (*scopedAuthRecorder) Refresh(context.Context, string) (auth.SessionResponse, error) {
	return auth.SessionResponse{}, nil
}

func (s projectScopeResolverStub) ResolveProjectHost(_ context.Context, host string) (project.Project, error) {
	host = strings.Split(host, ":")[0]
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return project.Project{}, project.ErrNoProjectHost
	}
	resolved, ok := s.projects[parts[0]]
	if !ok {
		return project.Project{}, project.ErrNoProjectHost
	}
	return resolved, nil
}

func twoProjectResolver() projectScopeResolverStub {
	return projectScopeResolverStub{projects: map[string]project.Project{
		"alpha": {ID: "alpha", Name: "Alpha", Status: "ready", Scope: project.ResourceScope{Database: project.DatabaseScope{Name: "supadata_alpha", Role: "supadata_alpha_runtime", Schema: "public"}, Storage: project.StorageScope{Bucket: "supadata-alpha"}}},
		"beta":  {ID: "beta", Name: "Beta", Status: "ready", Scope: project.ResourceScope{Database: project.DatabaseScope{Name: "supadata_beta", Role: "supadata_beta_runtime", Schema: "public"}, Storage: project.StorageScope{Bucket: "supadata-beta"}}},
	}}
}

func TestTwoProjectHostnameRoutingUsesSeparateAuthScopes(t *testing.T) {
	authRecorder := &scopedAuthRecorder{}
	server := NewServer(ServerOptions{ProjectResolver: twoProjectResolver(), RequireProjectScope: true, Auth: authRecorder, APIKeys: APIKeyConfig{Anon: "anon-key"}})

	for _, host := range []string{"alpha.supabase.example.com", "beta.supabase.example.com", "alpha.supabase.example.com"} {
		request := httptest.NewRequest(http.MethodPost, "https://"+host+"/auth/v1/signup", strings.NewReader(`{"email":"user@example.com","password":"password-123456"}`))
		request.Header.Set("apikey", "anon-key")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("host %q status = %d, want 200: %s", host, response.Code, response.Body.String())
		}
	}

	want := []string{"alpha", "beta", "alpha"}
	if len(authRecorder.seen) != len(want) {
		t.Fatalf("auth scopes = %v, want %v", authRecorder.seen, want)
	}
	for i := range want {
		if authRecorder.seen[i] != want[i] {
			t.Fatalf("auth scope %d = %q, want %q", i, authRecorder.seen[i], want[i])
		}
	}
}

func TestTwoProjectHostnameRoutingUsesSeparateRPCDatabases(t *testing.T) {
	alphaDB, alphaMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer alphaDB.Close()
	betaDB, betaMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer betaDB.Close()

	router := database.NewRouter(nil)
	if err := router.Register("alpha", alphaDB); err != nil {
		t.Fatal(err)
	}
	if err := router.Register("beta", betaDB); err != nil {
		t.Fatal(err)
	}
	query := regexp.QuoteMeta(`SELECT * FROM "public"."project_marker"()`)
	expectProjectAnonTransaction(alphaMock, "alpha")
	alphaMock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"project"}).AddRow("alpha"))
	alphaMock.ExpectCommit()
	expectProjectAnonTransaction(betaMock, "beta")
	betaMock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"project"}).AddRow("beta"))
	betaMock.ExpectCommit()

	server := NewServer(ServerOptions{
		ProjectResolver:     twoProjectResolver(),
		DatabaseResolver:    router,
		RequireProjectScope: true,
		REST:                rest.NewHandler(nil, rest.HandlerOptions{APIKeys: rest.APIKeyConfig{Anon: "anon-key"}}),
	})
	for _, item := range []struct{ host, want string }{{"alpha.supabase.example.com", "alpha"}, {"beta.supabase.example.com", "beta"}} {
		request := httptest.NewRequest(http.MethodPost, "https://"+item.host+"/rest/v1/rpc/project_marker", strings.NewReader(`{}`))
		request.Header.Set("apikey", "anon-key")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"project":"`+item.want+`"`) {
			t.Fatalf("host %q response = %d %s", item.host, response.Code, response.Body.String())
		}
	}
	if err := alphaMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := betaMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTwoProjectHostnameRoutingUsesSeparateRESTDatabases(t *testing.T) {
	alphaDB, alphaMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer alphaDB.Close()
	betaDB, betaMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer betaDB.Close()

	router := database.NewRouter(nil)
	if err := router.Register("alpha", alphaDB); err != nil {
		t.Fatal(err)
	}
	if err := router.Register("beta", betaDB); err != nil {
		t.Fatal(err)
	}
	expectProjectAnonTransaction(alphaMock, "alpha")
	alphaMock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "public"."todos"`)).WillReturnRows(sqlmock.NewRows([]string{"project"}).AddRow("alpha"))
	alphaMock.ExpectCommit()
	expectProjectAnonTransaction(betaMock, "beta")
	betaMock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "public"."todos"`)).WillReturnRows(sqlmock.NewRows([]string{"project"}).AddRow("beta"))
	betaMock.ExpectCommit()

	server := NewServer(ServerOptions{
		ProjectResolver:     twoProjectResolver(),
		DatabaseResolver:    router,
		RequireProjectScope: true,
		REST:                rest.NewHandler(nil, rest.HandlerOptions{APIKeys: rest.APIKeyConfig{Anon: "anon-key"}}),
	})
	for _, item := range []struct{ host, want string }{{"alpha.supabase.example.com", "alpha"}, {"beta.supabase.example.com", "beta"}} {
		request := httptest.NewRequest(http.MethodGet, "https://"+item.host+"/rest/v1/todos", nil)
		request.Header.Set("apikey", "anon-key")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"project":"`+item.want+`"`) {
			t.Fatalf("host %q response = %d %s", item.host, response.Code, response.Body.String())
		}
	}
	if err := alphaMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := betaMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func expectProjectAnonTransaction(mock sqlmock.Sqlmock, projectID string) {
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT\s+set_config\('role'`).WithArgs("anon", sqlmock.AnyArg(), "", projectID).WillReturnResult(sqlmock.NewResult(0, 1))
}
