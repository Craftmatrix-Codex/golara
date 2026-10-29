package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                   int
	DataDir                string
	PublicHost             string
	ControlPlaneToken      string
	StudioAuthUsername     string
	StudioAuthPassword     string
	RequireProjectScope    bool
	AllowedOrigin          string
	DatabaseMode           string
	StorageMode            string
	StorageEndpoint        string
	StorageAccessKey       string
	StorageSecretKey       string
	StorageRegion          string
	StorageUseSSL          bool
	DatabaseURL            string
	ProjectDatabaseURLs    map[string]string
	PublicDatabaseHost     string
	PublicDatabasePort     int
	PublicDatabaseName     string
	PublicDatabaseUser     string
	PublicConnectionString string
	JWTSecret              string
	AuthIssuer             string
	AuthAutoConfirm        bool
	AnonKey                string
	ServiceRoleKey         string
	AuthEmailEnabled       bool
	AuthPhoneEnabled       bool
	AuthDisableSignup      bool
	SMSProvider            string
}

func (c Config) PublicOrigin() string {
	origin := url.URL{Scheme: "https", Host: c.PublicHost}
	hostname := origin.Hostname()
	if ip := net.ParseIP(hostname); hostname == "localhost" || ip != nil && ip.IsLoopback() {
		origin.Scheme = "http"
	}
	return origin.String()
}

func Load() Config {
	return Config{
		Port:                   envInt("SUPADATA_PORT", envInt("PORT", 8090)),
		DataDir:                envString("SUPADATA_DATA_DIR", "/var/lib/supadata"),
		PublicHost:             envString("SUPADATA_PUBLIC_HOST", "supabase.craftmatrix.org"),
		ControlPlaneToken:      os.Getenv("SUPADATA_CONTROL_PLANE_TOKEN"),
		StudioAuthUsername:     os.Getenv("SUPADATA_STUDIO_AUTH_USERNAME"),
		StudioAuthPassword:     os.Getenv("SUPADATA_STUDIO_AUTH_PASSWORD"),
		RequireProjectScope:    envBool("SUPADATA_REQUIRE_PROJECT_SCOPE", true),
		AllowedOrigin:          envString("SUPADATA_ALLOWED_ORIGIN", "*"),
		DatabaseMode:           envString("SUPADATA_DATABASE_MODE", "shared"),
		StorageMode:            envString("SUPADATA_STORAGE_MODE", "shared"),
		StorageEndpoint:        os.Getenv("SUPADATA_STORAGE_ENDPOINT"),
		StorageAccessKey:       os.Getenv("SUPADATA_STORAGE_ACCESS_KEY"),
		StorageSecretKey:       os.Getenv("SUPADATA_STORAGE_SECRET_KEY"),
		StorageRegion:          envString("SUPADATA_STORAGE_REGION", "us-east-1"),
		StorageUseSSL:          envBool("SUPADATA_STORAGE_USE_SSL", false),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		ProjectDatabaseURLs:    envStringMap("SUPADATA_PROJECT_DATABASE_URLS"),
		PublicDatabaseHost:     os.Getenv("SUPADATA_PUBLIC_DATABASE_HOST"),
		PublicDatabasePort:     envInt("SUPADATA_PUBLIC_DATABASE_PORT", 0),
		PublicDatabaseName:     os.Getenv("SUPADATA_PUBLIC_DATABASE_NAME"),
		PublicDatabaseUser:     os.Getenv("SUPADATA_PUBLIC_DATABASE_USER"),
		PublicConnectionString: publicConnectionString(),
		JWTSecret:              os.Getenv("JWT_SECRET"),
		AuthIssuer:             envString("GOTRUE_JWT_ISSUER", "https://supabase.craftmatrix.org/auth/v1"),
		AuthAutoConfirm:        envBool("ENABLE_EMAIL_AUTOCONFIRM", false),
		AnonKey:                os.Getenv("ANON_KEY"),
		ServiceRoleKey:         os.Getenv("SERVICE_ROLE_KEY"),
		AuthEmailEnabled:       envBool("ENABLE_EMAIL_SIGNUP", true),
		AuthPhoneEnabled:       envBool("ENABLE_PHONE_SIGNUP", false),
		AuthDisableSignup:      envBool("DISABLE_SIGNUP", false),
		SMSProvider:            envString("SMS_PROVIDER", ""),
	}
}

func (c Config) ResolveProjectDatabaseURLs(projectIDs []string) (map[string]string, error) {
	resolved := make(map[string]string, len(projectIDs))
	mode := c.DatabaseMode
	if mode == "" {
		mode = "shared"
	}
	if mode != "shared" && mode != "per-project" {
		return nil, errors.New("database mode must be shared or per-project")
	}
	for _, projectID := range projectIDs {
		url := c.ProjectDatabaseURLs[projectID]
		if url == "" && mode == "shared" {
			url = c.DatabaseURL
		}
		if url == "" {
			return nil, fmt.Errorf("database URL is not configured for project %q", projectID)
		}
		resolved[projectID] = url
	}
	if len(projectIDs) == 0 && c.DatabaseURL == "" && len(c.ProjectDatabaseURLs) == 0 {
		return map[string]string{}, nil
	}
	return resolved, nil
}

func publicConnectionString() string {
	if value := os.Getenv("SUPADATA_PUBLIC_DATABASE_CONNECTION_STRING"); value != "" {
		return sanitizeConnectionString(value)
	}
	value := os.Getenv("DATABASE_URL")
	if value == "" {
		value = os.Getenv("SUPADATA_DATABASE_URL")
	}
	if value == "" {
		return ""
	}
	return sanitizeConnectionString(value)
}

func sanitizeConnectionString(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	parsed.User = url.UserPassword(parsed.User.Username(), "***")
	return parsed.String()
}

func (c Config) PublicDatabaseDetails() (host string, port int, name, user, connectionString string) {
	host, port, name, user, connectionString = c.PublicDatabaseHost, c.PublicDatabasePort, c.PublicDatabaseName, c.PublicDatabaseUser, c.PublicConnectionString
	value := c.DatabaseURL
	if value == "" {
		value = os.Getenv("SUPADATA_DATABASE_URL")
	}
	parsed, err := url.Parse(value)
	if parsed != nil && err == nil {
		if host == "" {
			host = parsed.Hostname()
		}
		if port == 0 {
			port, _ = strconv.Atoi(parsed.Port())
		}
		if name == "" {
			name = strings.TrimPrefix(parsed.Path, "/")
		}
		if user == "" && parsed.User != nil {
			user = parsed.User.Username()
		}
		if connectionString == "" {
			connectionString = sanitizeConnectionString(value)
		}
	}
	if port == 0 {
		port = 5432
	}
	if connectionString == "" && host != "" && name != "" {
		connectionString = fmt.Sprintf("postgresql://%s:***@%s:%d/%s", url.PathEscape(user), host, port, url.PathEscape(name))
	}
	return
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envStringMap(key string) map[string]string {
	value := os.Getenv(key)
	if value == "" {
		return map[string]string{}
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(value), &parsed); err != nil || parsed == nil {
		return map[string]string{}
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
