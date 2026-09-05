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
)

func TestGETTableAgainstPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("SUPADATA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SUPADATA_TEST_DATABASE_URL is not configured")
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
