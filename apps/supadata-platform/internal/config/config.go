package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port              int
	DataDir           string
	PublicHost        string
	ControlPlaneToken string
	AllowedOrigin     string
	DatabaseMode      string
	StorageMode       string
	StorageEndpoint   string
	StorageAccessKey  string
	StorageSecretKey  string
	StorageRegion     string
	StorageUseSSL     bool
	DatabaseURL       string
	JWTSecret         string
	AuthIssuer        string
	AuthAutoConfirm   bool
	AnonKey           string
	ServiceRoleKey    string
	AuthEmailEnabled  bool
	AuthPhoneEnabled  bool
	AuthDisableSignup bool
	SMSProvider       string
}

func Load() Config {
	return Config{
		Port:              envInt("SUPADATA_PORT", envInt("PORT", 8090)),
		DataDir:           envString("SUPADATA_DATA_DIR", "/var/lib/supadata"),
		PublicHost:        envString("SUPADATA_PUBLIC_HOST", "13.140.160.208"),
		ControlPlaneToken: os.Getenv("SUPADATA_CONTROL_PLANE_TOKEN"),
		AllowedOrigin:     envString("SUPADATA_ALLOWED_ORIGIN", "*"),
		DatabaseMode:      envString("SUPADATA_DATABASE_MODE", "shared"),
		StorageMode:       envString("SUPADATA_STORAGE_MODE", "shared"),
		StorageEndpoint:   os.Getenv("SUPADATA_STORAGE_ENDPOINT"),
		StorageAccessKey:  os.Getenv("SUPADATA_STORAGE_ACCESS_KEY"),
		StorageSecretKey:  os.Getenv("SUPADATA_STORAGE_SECRET_KEY"),
		StorageRegion:     envString("SUPADATA_STORAGE_REGION", "us-east-1"),
		StorageUseSSL:     envBool("SUPADATA_STORAGE_USE_SSL", false),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		AuthIssuer:        envString("GOTRUE_JWT_ISSUER", "https://supabase.craftmatrix.org/auth/v1"),
		AuthAutoConfirm:   envBool("ENABLE_EMAIL_AUTOCONFIRM", false),
		AnonKey:           os.Getenv("ANON_KEY"),
		ServiceRoleKey:    os.Getenv("SERVICE_ROLE_KEY"),
		AuthEmailEnabled:  envBool("ENABLE_EMAIL_SIGNUP", true),
		AuthPhoneEnabled:  envBool("ENABLE_PHONE_SIGNUP", false),
		AuthDisableSignup: envBool("DISABLE_SIGNUP", false),
		SMSProvider:       envString("SMS_PROVIDER", ""),
	}
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
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
