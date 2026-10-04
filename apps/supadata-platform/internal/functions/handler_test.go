package functions

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

type recordingExecutor struct {
	invocation  Invocation
	response    Result
	err         error
	requiresJWT bool
}

func (e *recordingExecutor) Invoke(_ context.Context, invocation Invocation) (Result, error) {
	e.invocation = invocation
	return e.response, e.err
}

func (e *recordingExecutor) RequiresJWT(_ context.Context, _, _ string) (bool, error) {
	return e.requiresJWT, e.err
}

func TestHandlerInvokesProjectScopedFunction(t *testing.T) {
	executor := &recordingExecutor{response: Result{
		Status:  http.StatusCreated,
		Headers: http.Header{"Content-Type": []string{"application/json"}, "X-Function": []string{"hello"}},
		Body:    []byte(`{"message":"hello"}`),
	}}
	handler := NewHandler(HandlerOptions{
		Executor: executor,
		APIKeys:  APIKeyConfig{Anon: "anon-key", ServiceRole: "service-key"},
	})
	request := httptest.NewRequest(http.MethodPost, "/functions/v1/hello?name=Anakin", strings.NewReader(`{"ok":true}`))
	request.Header.Set("apikey", "anon-key")
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(project.WithScope(request.Context(), project.Project{ID: "default"}))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Function") != "hello" {
		t.Fatalf("expected function response headers to be preserved")
	}
	if executor.invocation.ProjectID != "default" || executor.invocation.Slug != "hello" {
		t.Fatalf("unexpected invocation scope: %#v", executor.invocation)
	}
	if executor.invocation.Method != http.MethodPost || executor.invocation.RawQuery != "name=Anakin" {
		t.Fatalf("unexpected request metadata: %#v", executor.invocation)
	}
	body, _ := io.ReadAll(executor.invocation.Body)
	if string(body) != `{"ok":true}` {
		t.Fatalf("unexpected invocation body: %q", body)
	}
}

func TestHandlerRejectsMissingAPIKey(t *testing.T) {
	handler := NewHandler(HandlerOptions{Executor: &recordingExecutor{}, APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodGet, "/functions/v1/hello", nil)
	request = request.WithContext(project.WithScope(request.Context(), project.Project{ID: "default"}))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestHandlerRejectsUnsafeSlug(t *testing.T) {
	handler := NewHandler(HandlerOptions{Executor: &recordingExecutor{}, APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodGet, "/functions/v1/%2e%2e", nil)
	request.Header.Set("apikey", "anon-key")
	request = request.WithContext(project.WithScope(request.Context(), project.Project{ID: "default"}))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

func TestHandlerReturnsNotFoundForMissingFunction(t *testing.T) {
	executor := &recordingExecutor{err: errFunctionNotFound}
	handler := NewHandler(HandlerOptions{Executor: executor, APIKeys: APIKeyConfig{Anon: "anon-key"}})
	request := httptest.NewRequest(http.MethodGet, "/functions/v1/missing", nil)
	request.Header.Set("apikey", "anon-key")
	request = request.WithContext(project.WithScope(request.Context(), project.Project{ID: "default"}))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", response.Code, response.Body.String())
	}
	if !errors.Is(executor.err, errFunctionNotFound) {
		t.Fatal("fixture must use the function not-found error")
	}
}

func TestHandlerEnforcesFunctionJWTPolicy(t *testing.T) {
	secret := []byte("edge-function-secret")
	executor := &recordingExecutor{requiresJWT: true, response: Result{Status: http.StatusOK}}
	handler := NewHandler(HandlerOptions{Executor: executor, APIKeys: APIKeyConfig{Anon: "anon-key"}, JWTSecret: secret})
	request := httptest.NewRequest(http.MethodGet, "/functions/v1/protected", nil)
	request.Header.Set("apikey", "anon-key")
	request = request.WithContext(project.WithScope(request.Context(), project.Project{ID: "default"}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected missing JWT to return 401, got %d", response.Code)
	}

	token, err := jwt.SignHS256(jwt.Claims{ProjectID: "default", ExpiresAt: time.Now().Add(time.Minute).Unix()}, secret)
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, "/functions/v1/protected", nil)
	request.Header.Set("apikey", "anon-key")
	request.Header.Set("Authorization", "Bearer "+token)
	request = request.WithContext(project.WithScope(request.Context(), project.Project{ID: "default"}))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected valid JWT to pass, got %d: %s", response.Code, response.Body.String())
	}
}
