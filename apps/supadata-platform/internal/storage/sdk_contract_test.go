package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSDKMultipartUploadStoresOnlyTheFileAndListUsesRelativeNames(t *testing.T) {
	store := newMemoryStore()
	h := NewHandler(HandlerOptions{Store: store, APIKeys: APIKeyConfig{Anon: "anon", ServiceRole: "service"}})
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("cacheControl", "3600")
	part, _ := writer.CreateFormFile("", "sdk.txt")
	_, _ = io.WriteString(part, "SDK file content")
	_ = writer.Close()
	r := httptest.NewRequest("POST", "/storage/v1/object/avatars/contracts/sdk.txt", &body)
	r.Header.Set("apikey", "service")
	r.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	object, err := store.Get(context.Background(), "avatars", "contracts/sdk.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer object.Body.Close()
	content, _ := io.ReadAll(object.Body)
	if string(content) != "SDK file content" {
		t.Fatalf("stored multipart envelope instead of file: %q", content)
	}
	r = httptest.NewRequest("POST", "/storage/v1/object/list/avatars", strings.NewReader(`{"prefix":"contracts"}`))
	r.Header.Set("apikey", "service")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var files []ObjectInfo
	if err := json.Unmarshal(w.Body.Bytes(), &files); err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Key != "sdk.txt" {
		t.Fatalf("list=%s", w.Body.String())
	}
}

func TestSDKBatchRemoveValidatesEveryPathBeforeDeleting(t *testing.T) {
	store := newMemoryStore()
	h := NewHandler(HandlerOptions{Store: store, APIKeys: APIKeyConfig{ServiceRole: "service"}})
	_, _ = store.Put(context.Background(), "avatars", "contracts/sdk.txt", "text/plain", strings.NewReader("file"), 4)
	for _, tc := range []struct {
		body string
		want int
	}{{`{"prefixes":["contracts/sdk.txt","../escape"]}`, 400}, {`{"prefixes":["contracts/sdk.txt"]}`, 200}} {
		r := httptest.NewRequest("DELETE", "/storage/v1/object/avatars", strings.NewReader(tc.body))
		r.Header.Set("apikey", "service")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
	if _, err := store.Get(context.Background(), "avatars", "contracts/sdk.txt"); err != ErrObjectNotFound {
		t.Fatalf("object still exists: %v", err)
	}
}

func TestPublicObjectRouteDoesNotPermitAnonymousWrites(t *testing.T) {
	h := NewHandler(HandlerOptions{Store: newMemoryStore(), APIKeys: APIKeyConfig{Anon: "anon"}})
	r := httptest.NewRequest("POST", "/storage/v1/object/public/avatars/evil.txt", strings.NewReader("unauthorized"))
	r.Header.Set("apikey", "anon")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("status=%d, want 401", w.Code)
	}
}
