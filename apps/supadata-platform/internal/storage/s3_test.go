package storage

import "testing"

func TestNewS3StoreRequiresEndpointAndCredentials(t *testing.T) {
	for _, config := range []S3Config{
		{},
		{Endpoint: "seaweedfs:8333"},
		{Endpoint: "seaweedfs:8333", AccessKey: "access"},
	} {
		if _, err := NewS3Store(config); err == nil {
			t.Fatalf("NewS3Store(%+v) unexpectedly succeeded", config)
		}
	}
}

func TestNewS3StoreAcceptsS3CompatibleEndpoint(t *testing.T) {
	store, err := NewS3Store(S3Config{Endpoint: "127.0.0.1:8333", AccessKey: "access", SecretKey: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}
	if store == nil || store.client == nil {
		t.Fatal("NewS3Store() returned an empty store")
	}
}
