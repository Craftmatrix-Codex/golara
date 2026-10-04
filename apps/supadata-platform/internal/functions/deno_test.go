package functions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDenoExecutorInvokesDeployedFunction(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-specific")
	}
	root := t.TempDir()
	functionDir := filepath.Join(root, "default", "hello")
	if err := os.MkdirAll(functionDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(functionDir, "index.ts"), []byte("export default () => new Response('ok')"), 0o640); err != nil {
		t.Fatal(err)
	}
	manifest := `{"entrypoint_path":"index.ts"}`
	if err := os.WriteFile(filepath.Join(functionDir, "manifest.json"), []byte(manifest), 0o640); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(root, "wrapper.ts")
	if err := os.WriteFile(wrapper, []byte("fixture"), 0o640); err != nil {
		t.Fatal(err)
	}
	responseBody := base64.StdEncoding.EncodeToString([]byte(`{"ok":true}`))
	runtimePath := filepath.Join(root, "fake-deno")
	script := "#!/bin/sh\ncat >/dev/null\nprintf '%s\\n' '" + `{"status":202,"headers":{"Content-Type":["application/json"]},"body":"` + responseBody + `"}` + "'\n"
	if err := os.WriteFile(runtimePath, []byte(script), 0o750); err != nil {
		t.Fatal(err)
	}
	executor := NewDenoExecutor(DenoExecutorOptions{RootDir: root, RuntimePath: runtimePath, WrapperPath: wrapper, Timeout: time.Second})

	result, err := executor.Invoke(context.Background(), Invocation{
		ProjectID: "default",
		Slug:      "hello",
		Method:    http.MethodPost,
		RawQuery:  "name=Anakin",
		Headers:   http.Header{"Content-Type": []string{"application/json"}},
		Body:      strings.NewReader(`{"input":true}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != http.StatusAccepted || string(result.Body) != `{"ok":true}` {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Headers.Get("Content-Type") != "application/json" {
		t.Fatalf("response headers were not decoded: %#v", result.Headers)
	}
}

func TestDenoExecutorReturnsNotFoundForMissingFunction(t *testing.T) {
	executor := NewDenoExecutor(DenoExecutorOptions{RootDir: t.TempDir(), RuntimePath: "deno", WrapperPath: "wrapper.ts"})
	_, err := executor.Invoke(context.Background(), Invocation{ProjectID: "default", Slug: "missing"})
	if err == nil || !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestDenoExecutorReadsJWTPolicyFromManifest(t *testing.T) {
	root := t.TempDir()
	for slug, manifest := range map[string]string{
		"default-policy": `{"entrypoint_path":"index.ts"}`,
		"public-policy":  `{"entrypoint_path":"index.ts","verify_jwt":false}`,
	} {
		dir := filepath.Join(root, "default", slug)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	executor := NewDenoExecutor(DenoExecutorOptions{RootDir: root})
	required, err := executor.RequiresJWT(context.Background(), "default", "default-policy")
	if err != nil || !required {
		t.Fatalf("expected omitted verify_jwt to default true, required=%v err=%v", required, err)
	}
	required, err = executor.RequiresJWT(context.Background(), "default", "public-policy")
	if err != nil || required {
		t.Fatalf("expected verify_jwt=false, required=%v err=%v", required, err)
	}
}

func TestDenoExecutorRejectsManifestTraversal(t *testing.T) {
	root := t.TempDir()
	functionDir := filepath.Join(root, "default", "unsafe")
	if err := os.MkdirAll(functionDir, 0o750); err != nil {
		t.Fatal(err)
	}
	manifestBytes, _ := json.Marshal(map[string]string{"entrypoint_path": "../secret.ts"})
	if err := os.WriteFile(filepath.Join(functionDir, "manifest.json"), manifestBytes, 0o640); err != nil {
		t.Fatal(err)
	}
	executor := NewDenoExecutor(DenoExecutorOptions{RootDir: root, RuntimePath: "deno", WrapperPath: "wrapper.ts"})
	_, err := executor.Invoke(context.Background(), Invocation{ProjectID: "default", Slug: "unsafe"})
	if err == nil || !strings.Contains(err.Error(), "entrypoint") {
		t.Fatalf("expected unsafe entrypoint error, got %v", err)
	}
}

func TestDenoExecutorDoesNotInheritPlatformEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable fixture is Unix-specific")
	}
	t.Setenv("PLATFORM_SECRET", "must-not-leak")
	root := t.TempDir()
	functionDir := filepath.Join(root, "default", "environment")
	if err := os.MkdirAll(functionDir, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"index.ts":      "export default () => new Response('ok')",
		"manifest.json": `{"entrypoint_path":"index.ts"}`,
		"wrapper.ts":    "fixture",
	} {
		path := filepath.Join(functionDir, name)
		if name == "wrapper.ts" {
			path = filepath.Join(root, name)
		}
		if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	runtimePath := filepath.Join(root, "fake-runtime.py")
	script := `#!/usr/bin/python3
import base64, json, os, sys
sys.stdin.read()
value = ("exposed" if os.getenv("PLATFORM_SECRET") else "redacted") + ":" + (os.getenv("FUNCTION_SECRET") or "missing")
print(json.dumps({"status": 200, "headers": {}, "body": base64.b64encode(value.encode()).decode()}))
`
	if err := os.WriteFile(runtimePath, []byte(script), 0o750); err != nil {
		t.Fatal(err)
	}
	executor := NewDenoExecutor(DenoExecutorOptions{
		RootDir: root, RuntimePath: runtimePath, WrapperPath: filepath.Join(root, "wrapper.ts"),
		Environment: map[string]string{"FUNCTION_SECRET": "available"},
	})
	result, err := executor.Invoke(context.Background(), Invocation{
		ProjectID: "default", Slug: "environment", Method: http.MethodGet, Body: strings.NewReader(""),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Body) != "redacted:available" {
		t.Fatalf("unexpected runtime environment: %s", result.Body)
	}
}
