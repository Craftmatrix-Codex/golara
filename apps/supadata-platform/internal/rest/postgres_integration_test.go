package rest

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
)

func TestGETTableWithJWTUsesPostgresRLSClaims(t *testing.T) {
	databaseURL := os.Getenv("SUPADATA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SUPADATA_TEST_DATABASE_URL is not set")
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = database.ExecContext(ctx, `
		DROP TABLE IF EXISTS public.rls_todos;
		DROP ROLE IF EXISTS authenticated;
		CREATE ROLE authenticated NOLOGIN;
		GRANT authenticated TO CURRENT_USER;
		CREATE TABLE public.rls_todos (id integer primary key, owner text not null, title text not null);
		GRANT SELECT ON public.rls_todos TO authenticated;
		ALTER TABLE public.rls_todos ENABLE ROW LEVEL SECURITY;
		CREATE POLICY owner_can_read ON public.rls_todos FOR SELECT TO authenticated USING (owner = current_setting('request.jwt.claim.sub', true));
		INSERT INTO public.rls_todos VALUES (1, 'user-id', 'visible'), (2, 'other-user', 'hidden');
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer database.ExecContext(context.Background(), `DROP TABLE IF EXISTS public.rls_todos; DROP ROLE IF EXISTS authenticated;`)
	accessToken, err := jwt.SignHS256(jwt.Claims{Subject: "user-id", Role: "authenticated", ExpiresAt: time.Now().Add(time.Hour).Unix()}, []byte("rls-secret"))
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(database, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}, JWTSecret: []byte("rls-secret")})
	request := httptest.NewRequest(http.MethodGet, "/rest/v1/rls_todos?select=id,title", nil)
	request.Header.Set("apikey", "anon-key")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != `[{"id":1,"title":"visible"}]
` {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}

func TestGETTableAgainstPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("SUPADATA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SUPADATA_TEST_DATABASE_URL is not set")
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	if _, err := database.ExecContext(ctx, `DROP TABLE IF EXISTS public.todos; CREATE TABLE public.todos (id integer PRIMARY KEY, title text); INSERT INTO public.todos VALUES (42, 'Fix auth');`); err != nil {
		t.Fatalf("create REST fixture: %v", err)
	}

	handler := NewHandler(database, HandlerOptions{APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodGet, "/rest/v1/todos?select=id,title&id=eq.42", nil)
	request.Header.Set("apikey", "anon-key")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != `[{"id":42,"title":"Fix auth"}]`+"\n" {
		t.Fatalf("response status=%d body=%q", response.Code, response.Body.String())
	}
}
