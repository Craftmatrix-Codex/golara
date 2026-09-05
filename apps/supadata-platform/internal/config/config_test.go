package config

import "testing"

func TestLoadUsesProductionSafeDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("SUPADATA_DATA_DIR", "")
	t.Setenv("SUPADATA_PUBLIC_HOST", "")

	cfg := Load()
	if cfg.Port != 8090 {
		t.Fatalf("Port = %d, want 8090", cfg.Port)
	}
	if cfg.DataDir != "/var/lib/supadata" {
		t.Fatalf("DataDir = %q, want /var/lib/supadata", cfg.DataDir)
	}
	if cfg.PublicHost != "13.140.160.208" {
		t.Fatalf("PublicHost = %q, want configured production default", cfg.PublicHost)
	}
}

func TestLoadReadsExplicitValues(t *testing.T) {
	t.Setenv("PORT", "9100")
	t.Setenv("SUPADATA_DATA_DIR", "/tmp/supadata")
	t.Setenv("SUPADATA_PUBLIC_HOST", "203.0.113.10")
	t.Setenv("SUPADATA_CONTROL_PLANE_TOKEN", "configured-token")

	cfg := Load()
	if cfg.Port != 9100 || cfg.DataDir != "/tmp/supadata" || cfg.PublicHost != "203.0.113.10" {
		t.Fatalf("Load() did not preserve explicit configuration: %+v", cfg)
	}
	if cfg.ControlPlaneToken != "configured-token" {
		t.Fatal("Load() did not read the control-plane token")
	}
}

func TestLoadReadsS3StorageConfiguration(t *testing.T) {
	t.Setenv("SUPADATA_STORAGE_ENDPOINT", "seaweedfs:8333")
	t.Setenv("SUPADATA_STORAGE_ACCESS_KEY", "access")
	t.Setenv("SUPADATA_STORAGE_SECRET_KEY", "secret")
	t.Setenv("SUPADATA_STORAGE_REGION", "us-east-1")
	t.Setenv("SUPADATA_STORAGE_USE_SSL", "true")

	cfg := Load()
	if cfg.StorageEndpoint != "seaweedfs:8333" || cfg.StorageAccessKey != "access" || cfg.StorageSecretKey != "secret" || cfg.StorageRegion != "us-east-1" || !cfg.StorageUseSSL {
		t.Fatalf("Load() did not read storage configuration: %+v", cfg)
	}
}
