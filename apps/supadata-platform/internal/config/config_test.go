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
	if cfg.PublicHost != "supabase.craftmatrix.org" {
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

func TestLoadReadsProjectDatabaseRoutingAndFailsClosedByDefault(t *testing.T) {
	t.Setenv("SUPADATA_REQUIRE_PROJECT_SCOPE", "")
	t.Setenv("SUPADATA_PROJECT_DATABASE_URLS", `{"alpha":"postgres://alpha","beta":"postgres://beta"}`)

	cfg := Load()
	if !cfg.RequireProjectScope {
		t.Fatal("RequireProjectScope must default to true")
	}
	if cfg.ProjectDatabaseURLs["alpha"] != "postgres://alpha" || cfg.ProjectDatabaseURLs["beta"] != "postgres://beta" {
		t.Fatalf("ProjectDatabaseURLs = %#v", cfg.ProjectDatabaseURLs)
	}
}

func TestResolveProjectDatabaseURLsRequiresEveryProjectInPerProjectMode(t *testing.T) {
	cfg := Config{DatabaseMode: "per-project", ProjectDatabaseURLs: map[string]string{"alpha": "postgres://alpha"}}
	_, err := cfg.ResolveProjectDatabaseURLs([]string{"alpha", "beta"})
	if err == nil {
		t.Fatal("per-project mode must reject a missing project database URL")
	}
}

func TestResolveProjectDatabaseURLsMapsSharedDatabaseExplicitly(t *testing.T) {
	cfg := Config{DatabaseMode: "shared", DatabaseURL: "postgres://shared"}
	urls, err := cfg.ResolveProjectDatabaseURLs([]string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if urls["alpha"] != "postgres://shared" || urls["beta"] != "postgres://shared" {
		t.Fatalf("resolved URLs = %#v", urls)
	}
}

func TestLoadRejectsInvalidProjectDatabaseRoutingConfiguration(t *testing.T) {
	t.Setenv("SUPADATA_PROJECT_DATABASE_URLS", "not-json")
	cfg := Load()
	if len(cfg.ProjectDatabaseURLs) != 0 {
		t.Fatalf("invalid routing configuration should be ignored, got %#v", cfg.ProjectDatabaseURLs)
	}
}

func TestPublicOriginUsesHTTPSExceptExplicitLoopbackDevelopment(t *testing.T) {
	for _, tc := range []struct{ host, want string }{
		{"golara.craftmatrix.org", "https://golara.craftmatrix.org"},
		{"127.0.0.1:18080", "http://127.0.0.1:18080"},
		{"localhost:18080", "http://localhost:18080"},
		{"[::1]:18080", "http://[::1]:18080"},
		{"203.0.113.10:8080", "https://203.0.113.10:8080"},
	} {
		if got := (Config{PublicHost: tc.host}).PublicOrigin(); got != tc.want {
			t.Fatalf("host=%s got=%s want=%s", tc.host, got, tc.want)
		}
	}
}
