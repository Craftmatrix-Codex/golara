package functions

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var errFunctionNotFound = errors.New("function not found")

func IsNotFound(err error) bool {
	return errors.Is(err, errFunctionNotFound)
}

type DenoExecutorOptions struct {
	RootDir     string
	RuntimePath string
	WrapperPath string
	Timeout     time.Duration
	Environment map[string]string
}

type DenoExecutor struct {
	rootDir     string
	runtimePath string
	wrapperPath string
	timeout     time.Duration
	environment map[string]string
}

type functionManifest struct {
	EntrypointPath string `json:"entrypoint_path"`
	VerifyJWT      *bool  `json:"verify_jwt"`
}

type runtimeInvocation struct {
	Method   string              `json:"method"`
	URL      string              `json:"url"`
	Headers  map[string][]string `json:"headers"`
	Body     string              `json:"body"`
	Function string              `json:"function"`
	Project  string              `json:"project"`
}

type runtimeResult struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

func NewDenoExecutor(options DenoExecutorOptions) *DenoExecutor {
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &DenoExecutor{
		rootDir:     options.RootDir,
		runtimePath: options.RuntimePath,
		wrapperPath: options.WrapperPath,
		timeout:     timeout,
		environment: cloneEnvironment(options.Environment),
	}
}

func (e *DenoExecutor) RequiresJWT(_ context.Context, projectID, slug string) (bool, error) {
	_, manifest, err := e.readManifest(projectID, slug)
	if err != nil {
		return false, err
	}
	return manifest.VerifyJWT == nil || *manifest.VerifyJWT, nil
}

func (e *DenoExecutor) Invoke(ctx context.Context, invocation Invocation) (Result, error) {
	functionDir, manifest, err := e.readManifest(invocation.ProjectID, invocation.Slug)
	if err != nil {
		return Result{}, err
	}
	entrypoint, err := safeEntrypoint(functionDir, manifest.EntrypointPath)
	if err != nil {
		return Result{}, err
	}
	if _, err := os.Stat(entrypoint); errors.Is(err, os.ErrNotExist) {
		return Result{}, errFunctionNotFound
	} else if err != nil {
		return Result{}, fmt.Errorf("stat function entrypoint: %w", err)
	}
	body, err := io.ReadAll(io.LimitReader(invocation.Body, 10<<20))
	if err != nil {
		return Result{}, fmt.Errorf("read invocation body: %w", err)
	}
	payload, err := json.Marshal(runtimeInvocation{
		Method:   invocation.Method,
		URL:      "/functions/v1/" + invocation.Slug + querySuffix(invocation.RawQuery),
		Headers:  invocation.Headers,
		Body:     base64.StdEncoding.EncodeToString(body),
		Function: invocation.Slug,
		Project:  invocation.ProjectID,
	})
	if err != nil {
		return Result{}, fmt.Errorf("encode invocation: %w", err)
	}
	invocationContext, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	command := exec.CommandContext(invocationContext, e.runtimePath,
		"run", "--quiet", "--no-prompt", "--allow-env", "--allow-net",
		"--allow-read="+e.wrapperPath+","+functionDir,
		e.wrapperPath, entrypoint,
	)
	command.Stdin = bytes.NewReader(payload)
	command.Env = environmentList(e.environment)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if errors.Is(invocationContext.Err(), context.DeadlineExceeded) {
			return Result{}, errors.New("function invocation timed out")
		}
		return Result{}, fmt.Errorf("function runtime failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var decoded runtimeResult
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &decoded); err != nil {
		return Result{}, fmt.Errorf("decode function response: %w", err)
	}
	responseBody, err := base64.StdEncoding.DecodeString(decoded.Body)
	if err != nil {
		return Result{}, fmt.Errorf("decode function body: %w", err)
	}
	return Result{Status: decoded.Status, Headers: http.Header(decoded.Headers), Body: responseBody}, nil
}

func (e *DenoExecutor) readManifest(projectID, slug string) (string, functionManifest, error) {
	if !functionSlugPattern.MatchString(projectID) || !functionSlugPattern.MatchString(slug) {
		return "", functionManifest{}, errors.New("invalid function scope")
	}
	functionDir := filepath.Join(e.rootDir, projectID, slug)
	manifestBytes, err := os.ReadFile(filepath.Join(functionDir, "manifest.json"))
	if errors.Is(err, os.ErrNotExist) {
		return "", functionManifest{}, errFunctionNotFound
	}
	if err != nil {
		return "", functionManifest{}, fmt.Errorf("read function manifest: %w", err)
	}
	var manifest functionManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return "", functionManifest{}, fmt.Errorf("decode function manifest: %w", err)
	}
	return functionDir, manifest, nil
}

func cloneEnvironment(environment map[string]string) map[string]string {
	cloned := make(map[string]string, len(environment))
	for name, value := range environment {
		cloned[name] = value
	}
	return cloned
}

func environmentList(environment map[string]string) []string {
	names := make([]string, 0, len(environment))
	for name := range environment {
		names = append(names, name)
	}
	sort.Strings(names)
	values := make([]string, 0, len(names))
	for _, name := range names {
		values = append(values, name+"="+environment[name])
	}
	return values
}

func safeEntrypoint(functionDir, configured string) (string, error) {
	if configured == "" {
		configured = "index.ts"
	}
	cleaned := filepath.Clean(filepath.FromSlash(configured))
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid function entrypoint")
	}
	entrypoint := filepath.Join(functionDir, cleaned)
	relative, err := filepath.Rel(functionDir, entrypoint)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid function entrypoint")
	}
	return entrypoint, nil
}

func querySuffix(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	return "?" + rawQuery
}
