package storage

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"

	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
)

var ErrObjectNotFound = errors.New("object not found")
var ErrObjectTooLarge = errors.New("object too large")

type ObjectInfo struct {
	Bucket       string    `json:"-"`
	Key          string    `json:"name"`
	ETag         string    `json:"etag,omitempty"`
	ContentType  string    `json:"content_type,omitempty"`
	Size         int64     `json:"size,omitempty"`
	LastModified time.Time `json:"last_modified,omitempty"`
}

type ObjectReader struct {
	ObjectInfo
	Body io.ReadCloser
}

type ObjectStore interface {
	Put(context.Context, string, string, string, io.Reader, int64) (ObjectInfo, error)
	Get(context.Context, string, string) (ObjectReader, error)
	Delete(context.Context, string, string) error
	List(context.Context, string, string, int, int) ([]ObjectInfo, error)
}

type APIKeyConfig struct {
	Anon        string
	ServiceRole string
}

type HandlerOptions struct {
	Store         ObjectStore
	APIKeys       APIKeyConfig
	JWTSecret     []byte
	Issuer        string
	Audience      string
	MaxObjectSize int64
}

type Handler struct {
	store         ObjectStore
	apiKeys       APIKeyConfig
	jwtSecret     []byte
	issuer        string
	audience      string
	maxObjectSize int64
}

func NewHandler(options HandlerOptions) *Handler {
	maxSize := options.MaxObjectSize
	if maxSize <= 0 {
		maxSize = 50 << 20
	}
	return &Handler{
		store:         options.Store,
		apiKeys:       options.APIKeys,
		jwtSecret:     append([]byte(nil), options.JWTSecret...),
		issuer:        options.Issuer,
		audience:      options.Audience,
		maxObjectSize: maxSize,
	}
}

func (h *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if h.store == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "storage unavailable"})
		return
	}
	role := h.apiKeyRole(request.Header.Get("apikey"))
	if role == "" {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodPost && request.Method != http.MethodDelete {
		writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	path := strings.TrimPrefix(request.URL.Path, "/storage/v1/object/")
	if path == request.URL.Path || path == "" {
		writeJSON(response, http.StatusNotFound, map[string]string{"error": "object route not found"})
		return
	}
	if strings.HasPrefix(path, "list/") && request.Method == http.MethodPost {
		h.handleList(response, request, strings.TrimPrefix(path, "list/"))
		return
	}
	public, bucket, key, err := parseObjectPath(path)
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !public && request.Method != http.MethodGet && !h.hasAuthenticatedMutation(request, role) {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	switch request.Method {
	case http.MethodGet:
		h.handleGet(response, request, bucket, key)
	case http.MethodPost:
		h.handlePut(response, request, bucket, key)
	case http.MethodDelete:
		h.handleDelete(response, request, bucket, key)
	}
}

func (h *Handler) handlePut(response http.ResponseWriter, request *http.Request, bucket, key string) {
	if request.ContentLength > h.maxObjectSize {
		writeJSON(response, http.StatusRequestEntityTooLarge, map[string]string{"error": "object too large"})
		return
	}
	body := io.Reader(request.Body)
	if request.ContentLength < 0 {
		body = &maxSizeReader{reader: request.Body, remaining: h.maxObjectSize}
	}
	info, err := h.store.Put(request.Context(), bucket, key, request.Header.Get("Content-Type"), body, request.ContentLength)
	if err != nil {
		if errors.Is(err, ErrObjectTooLarge) {
			_ = h.store.Delete(request.Context(), bucket, key)
			writeJSON(response, http.StatusRequestEntityTooLarge, map[string]string{"error": "object too large"})
			return
		}
		writeStoreError(response, err)
		return
	}
	if info.Bucket == "" {
		info.Bucket = bucket
	}
	if info.Key == "" {
		info.Key = key
	}
	writeJSON(response, http.StatusOK, map[string]string{"Key": info.Bucket + "/" + info.Key})
}

type maxSizeReader struct {
	reader    io.Reader
	remaining int64
}

func (r *maxSizeReader) Read(destination []byte) (int, error) {
	if r.remaining == 0 {
		var probe [1]byte
		n, err := r.reader.Read(probe[:])
		if n > 0 {
			return 0, ErrObjectTooLarge
		}
		return 0, err
	}
	if int64(len(destination)) > r.remaining {
		destination = destination[:r.remaining]
	}
	read, err := r.reader.Read(destination)
	r.remaining -= int64(read)
	return read, err
}

func (h *Handler) handleGet(response http.ResponseWriter, request *http.Request, bucket, key string) {
	object, err := h.store.Get(request.Context(), bucket, key)
	if err != nil {
		writeStoreError(response, err)
		return
	}
	defer object.Body.Close()
	if object.ContentType != "" {
		response.Header().Set("Content-Type", object.ContentType)
	}
	if object.ETag != "" {
		response.Header().Set("ETag", object.ETag)
	}
	if object.Size >= 0 {
		response.Header().Set("Content-Length", strconv.FormatInt(object.Size, 10))
	}
	if _, err := io.Copy(response, object.Body); err != nil {
		return
	}
}

func (h *Handler) handleDelete(response http.ResponseWriter, request *http.Request, bucket, key string) {
	if err := h.store.Delete(request.Context(), bucket, key); err != nil {
		writeStoreError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"message": "Successfully deleted"})
}

func (h *Handler) handleList(response http.ResponseWriter, request *http.Request, bucket string) {
	if err := validateBucket(bucket); err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var body struct {
		Prefix string `json:"prefix"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if request.Body != nil && request.ContentLength != 0 {
		if err := json.NewDecoder(http.MaxBytesReader(response, request.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid list body"})
			return
		}
	}
	if err := validateObjectPath(body.Prefix, true); err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if body.Limit <= 0 || body.Limit > 1000 {
		body.Limit = 1000
	}
	if body.Offset < 0 {
		body.Offset = 0
	}
	items, err := h.store.List(request.Context(), bucket, body.Prefix, body.Limit, body.Offset)
	if err != nil {
		writeStoreError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, items)
}

func (h *Handler) hasAuthenticatedMutation(request *http.Request, role string) bool {
	if role == "service_role" {
		return true
	}
	authorization := request.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "Bearer ") || len(h.jwtSecret) == 0 {
		return false
	}
	accessToken := strings.TrimPrefix(authorization, "Bearer ")
	claims, err := jwt.VerifyHS256(accessToken, h.jwtSecret, jwt.ValidationOptions{Now: time.Now(), Issuer: h.issuer, Audience: h.audience})
	return err == nil && claims.Role != "service_role"
}

func (h *Handler) apiKeyRole(provided string) string {
	if provided == "" {
		return ""
	}
	for _, candidate := range []struct {
		key  string
		role string
	}{
		{h.apiKeys.ServiceRole, "service_role"},
		{h.apiKeys.Anon, "anon"},
	} {
		if candidate.key != "" && len(candidate.key) == len(provided) && subtle.ConstantTimeCompare([]byte(candidate.key), []byte(provided)) == 1 {
			return candidate.role
		}
	}
	return ""
}

func parseObjectPath(path string) (bool, string, string, error) {
	public := false
	if strings.HasPrefix(path, "public/") {
		public = true
		path = strings.TrimPrefix(path, "public/")
	}
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false, "", "", errors.New("bucket and object path are required")
	}
	if err := validateBucket(parts[0]); err != nil {
		return false, "", "", err
	}
	if err := validateObjectPath(parts[1], false); err != nil {
		return false, "", "", err
	}
	return public, parts[0], parts[1], nil
}

func validateBucket(bucket string) error {
	if len(bucket) < 1 || len(bucket) > 63 {
		return errors.New("invalid bucket")
	}
	for index, character := range bucket {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' || character == '_' {
			if index == 0 && character == '-' {
				return errors.New("invalid bucket")
			}
			continue
		}
		return errors.New("invalid bucket")
	}
	return nil
}

func validateObjectPath(path string, allowPrefix bool) error {
	if path == "" && allowPrefix {
		return nil
	}
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "\\") || strings.ContainsRune(path, 0) {
		return errors.New("invalid object path")
	}
	segments := strings.Split(path, "/")
	for index, segment := range segments {
		if segment == "" && allowPrefix && index == len(segments)-1 && strings.HasSuffix(path, "/") {
			continue
		}
		if segment == "" || segment == "." || segment == ".." || strings.ContainsRune(segment, '\u0000') {
			return errors.New("invalid object path")
		}
	}
	return nil
}

func writeStoreError(response http.ResponseWriter, err error) {
	if errors.Is(err, ErrObjectNotFound) {
		writeJSON(response, http.StatusNotFound, map[string]string{"error": "object not found"})
		return
	}
	writeJSON(response, http.StatusInternalServerError, map[string]string{"error": "storage operation failed"})
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	if status != http.StatusNoContent {
		if err := json.NewEncoder(response).Encode(payload); err != nil {
			return
		}
	}
}
