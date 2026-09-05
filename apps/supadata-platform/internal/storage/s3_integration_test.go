package storage

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
)

func TestS3StoreRoundTrip(t *testing.T) {
	endpoint := os.Getenv("SUPADATA_STORAGE_IT_ENDPOINT")
	if endpoint == "" {
		t.Skip("SUPADATA_STORAGE_IT_ENDPOINT is not set")
	}
	store, err := NewS3Store(S3Config{Endpoint: endpoint, AccessKey: os.Getenv("SUPADATA_STORAGE_IT_ACCESS_KEY"), SecretKey: os.Getenv("SUPADATA_STORAGE_IT_SECRET_KEY"), Region: "us-east-1"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	bucket := "supadata-it"
	_ = store.client.RemoveBucket(ctx, bucket)
	if err := store.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: "us-east-1"}); err != nil {
		t.Fatal(err)
	}
	defer store.client.RemoveBucket(ctx, bucket)

	if _, err := store.Put(ctx, bucket, "folder/hello.txt", "text/plain", strings.NewReader("hello"), 5); err != nil {
		t.Fatal(err)
	}
	object, err := store.Get(ctx, bucket, "folder/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(object.Body)
	_ = object.Body.Close()
	if readErr != nil || string(data) != "hello" {
		t.Fatalf("read data=%q err=%v", data, readErr)
	}
	items, err := store.List(ctx, bucket, "folder/", 10, 0)
	if err != nil || len(items) != 1 || items[0].Key != "folder/hello.txt" {
		t.Fatalf("list items=%+v err=%v", items, err)
	}
	if err := store.Delete(ctx, bucket, "folder/hello.txt"); err != nil {
		t.Fatal(err)
	}
}
