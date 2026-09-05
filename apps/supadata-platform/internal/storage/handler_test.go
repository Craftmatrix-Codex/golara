package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
)

type memoryObject struct {
	contentType string
	data        []byte
}

type memoryStore struct {
	mu      sync.Mutex
	objects map[string]memoryObject
}

func newMemoryStore() *memoryStore {
	return &memoryStore{objects: make(map[string]memoryObject)}
}

func (m *memoryStore) Put(_ context.Context, bucket, key, contentType string, body io.Reader, _ int64) (ObjectInfo, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return ObjectInfo{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[bucket+"/"+key] = memoryObject{contentType: contentType, data: append([]byte(nil), data...)}
	return ObjectInfo{Bucket: bucket, Key: key, ContentType: contentType, Size: int64(len(data))}, nil
}

func (m *memoryStore) Get(_ context.Context, bucket, key string) (ObjectReader, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	object, ok := m.objects[bucket+"/"+key]
	if !ok {
		return ObjectReader{}, ErrObjectNotFound
	}
	return ObjectReader{ObjectInfo: ObjectInfo{Bucket: bucket, Key: key, ContentType: object.contentType, Size: int64(len(object.data))}, Body: io.NopCloser(bytes.NewReader(object.data))}, nil
}

func (m *memoryStore) Delete(_ context.Context, bucket, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.objects[bucket+"/"+key]; !ok {
		return ErrObjectNotFound
	}
	delete(m.objects, bucket+"/"+key)
	return nil
}

func (m *memoryStore) List(_ context.Context, bucket, prefix string, limit, offset int) ([]ObjectInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]ObjectInfo, 0)
	for full, object := range m.objects {
		parts := strings.SplitN(full, "/", 2)
		if len(parts) != 2 || parts[0] != bucket || !strings.HasPrefix(parts[1], prefix) {
			continue
		}
		items = append(items, ObjectInfo{Bucket: bucket, Key: parts[1], ContentType: object.contentType, Size: int64(len(object.data))})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	if offset > len(items) {
		offset = len(items)
	}
	items = items[offset:]
	if limit >= 0 && limit < len(items) {
		items = items[:limit]
	}
	return items, nil
}

func TestStorageUploadDownloadAndDelete(t *testing.T) {
	store := newMemoryStore()
	handler := NewHandler(HandlerOptions{Store: store, APIKeys: APIKeyConfig{Anon: "anon", ServiceRole: "service"}})

	upload := httptest.NewRequest(http.MethodPost, "/storage/v1/object/avatars/user-1.txt", strings.NewReader("hello"))
	upload.Header.Set("apikey", "service")
	upload.Header.Set("Content-Type", "text/plain")
	uploadResponse := httptest.NewRecorder()
	handler.ServeHTTP(uploadResponse, upload)
	if uploadResponse.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", uploadResponse.Code, uploadResponse.Body.String())
	}

	download := httptest.NewRequest(http.MethodGet, "/storage/v1/object/avatars/user-1.txt", nil)
	download.Header.Set("apikey", "anon")
	downloadResponse := httptest.NewRecorder()
	handler.ServeHTTP(downloadResponse, download)
	if downloadResponse.Code != http.StatusOK || downloadResponse.Body.String() != "hello" {
		t.Fatalf("download = status %d body %q", downloadResponse.Code, downloadResponse.Body.String())
	}
	if got := downloadResponse.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("content type = %q", got)
	}

	remove := httptest.NewRequest(http.MethodDelete, "/storage/v1/object/avatars/user-1.txt", nil)
	remove.Header.Set("apikey", "service")
	removeResponse := httptest.NewRecorder()
	handler.ServeHTTP(removeResponse, remove)
	if removeResponse.Code != http.StatusOK {
		t.Fatalf("remove status = %d, body = %s", removeResponse.Code, removeResponse.Body.String())
	}

	afterRemove := httptest.NewRecorder()
	handler.ServeHTTP(afterRemove, httptest.NewRequest(http.MethodGet, "/storage/v1/object/avatars/user-1.txt", nil))
	if afterRemove.Code != http.StatusUnauthorized {
		t.Fatalf("missing API key status = %d", afterRemove.Code)
	}
}

func TestStorageRequiresPrivilegedAuthorizationForAnonymousMutation(t *testing.T) {
	handler := NewHandler(HandlerOptions{Store: newMemoryStore(), APIKeys: APIKeyConfig{Anon: "anon", ServiceRole: "service"}})
	request := httptest.NewRequest(http.MethodPost, "/storage/v1/object/avatars/user-1.txt", strings.NewReader("hello"))
	request.Header.Set("apikey", "anon")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestStorageRejectsUnsafeObjectPath(t *testing.T) {
	handler := NewHandler(HandlerOptions{Store: newMemoryStore(), APIKeys: APIKeyConfig{ServiceRole: "service"}})
	request := httptest.NewRequest(http.MethodPost, "/storage/v1/object/avatars/../secrets.txt", strings.NewReader("secret"))
	request.Header.Set("apikey", "service")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestStorageListUsesPrefixLimitAndOffset(t *testing.T) {
	store := newMemoryStore()
	for _, key := range []string{"images/a.txt", "images/b.txt", "images/c.txt", "other.txt"} {
		if _, err := store.Put(context.Background(), "avatars", key, "text/plain", strings.NewReader(key), -1); err != nil {
			t.Fatal(err)
		}
	}
	handler := NewHandler(HandlerOptions{Store: store, APIKeys: APIKeyConfig{Anon: "anon"}})
	request := httptest.NewRequest(http.MethodPost, "/storage/v1/object/list/avatars", strings.NewReader(`{"prefix":"images/","limit":1,"offset":1}`))
	request.Header.Set("apikey", "anon")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"name":"images/b.txt"`) || strings.Contains(response.Body.String(), "images/a.txt") {
		t.Fatalf("unexpected list body = %s", response.Body.String())
	}
}

type unknownLengthReader struct {
	reader io.Reader
}

func (r unknownLengthReader) Read(destination []byte) (int, error) {
	return r.reader.Read(destination)
}

func TestStorageRejectsOversizedUnknownLengthBody(t *testing.T) {
	store := newMemoryStore()
	handler := NewHandler(HandlerOptions{Store: store, APIKeys: APIKeyConfig{ServiceRole: "service"}, MaxObjectSize: 5})
	request := httptest.NewRequest(http.MethodPost, "/storage/v1/object/avatars/too-large.txt", unknownLengthReader{reader: strings.NewReader("123456")})
	request.Header.Set("apikey", "service")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if _, err := store.Get(context.Background(), "avatars", "too-large.txt"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("oversized object was stored: %v", err)
	}
}
