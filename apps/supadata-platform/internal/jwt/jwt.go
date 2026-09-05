package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Claims struct {
	Subject      string         `json:"sub,omitempty"`
	Email        string         `json:"email,omitempty"`
	Phone        string         `json:"phone,omitempty"`
	Role         string         `json:"role,omitempty"`
	Audience     string         `json:"aud,omitempty"`
	Issuer       string         `json:"iss,omitempty"`
	SessionID    string         `json:"session_id,omitempty"`
	IssuedAt     int64          `json:"iat,omitempty"`
	ExpiresAt    int64          `json:"exp,omitempty"`
	AppMetadata  map[string]any `json:"app_metadata,omitempty"`
	UserMetadata map[string]any `json:"user_metadata,omitempty"`
}

type ValidationOptions struct {
	Now      time.Time
	Audience string
	Issuer   string
}

type tokenHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ,omitempty"`
}

type tokenClaims struct {
	Subject      string         `json:"sub"`
	Email        string         `json:"email"`
	Phone        string         `json:"phone"`
	Role         string         `json:"role"`
	Audience     string         `json:"aud"`
	Issuer       string         `json:"iss"`
	SessionID    string         `json:"session_id"`
	IssuedAt     json.Number    `json:"iat"`
	ExpiresAt    json.Number    `json:"exp"`
	AppMetadata  map[string]any `json:"app_metadata"`
	UserMetadata map[string]any `json:"user_metadata"`
}

func SignHS256(claims Claims, secret []byte) (string, error) {
	if len(secret) == 0 {
		return "", errors.New("JWT signing secret is required")
	}
	header, err := json.Marshal(tokenHeader{Algorithm: "HS256", Type: "JWT"})
	if err != nil {
		return "", fmt.Errorf("encode JWT header: %w", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode JWT claims: %w", err)
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	unsigned := encodedHeader + "." + encodedPayload
	return unsigned + "." + signature(unsigned, secret), nil
}

func VerifyHS256(token string, secret []byte, options ValidationOptions) (Claims, error) {
	if len(secret) == 0 {
		return Claims{}, errors.New("JWT verification secret is required")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Claims{}, errors.New("malformed JWT")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, errors.New("malformed JWT header")
	}
	var header tokenHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil || header.Algorithm != "HS256" {
		return Claims{}, errors.New("unsupported JWT algorithm")
	}

	expected := signature(parts[0]+"."+parts[1], secret)
	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(provided) != sha256.Size || subtle.ConstantTimeCompare(provided, mustDecode(expected)) != 1 {
		return Claims{}, errors.New("invalid JWT signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, errors.New("malformed JWT claims")
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	var raw tokenClaims
	if err := decoder.Decode(&raw); err != nil {
		return Claims{}, errors.New("malformed JWT claims")
	}
	expiresAt, err := parseUnixClaim(raw.ExpiresAt, "exp")
	if err != nil || expiresAt <= validationNow(options).Unix() {
		return Claims{}, errors.New("JWT is expired or missing exp")
	}
	if options.Audience != "" && raw.Audience != options.Audience {
		return Claims{}, errors.New("JWT audience mismatch")
	}
	if options.Issuer != "" && raw.Issuer != options.Issuer {
		return Claims{}, errors.New("JWT issuer mismatch")
	}
	issuedAt, err := parseOptionalUnixClaim(raw.IssuedAt)
	if err != nil {
		return Claims{}, errors.New("invalid JWT iat")
	}
	return Claims{
		Subject:      raw.Subject,
		Email:        raw.Email,
		Phone:        raw.Phone,
		Role:         raw.Role,
		Audience:     raw.Audience,
		Issuer:       raw.Issuer,
		SessionID:    raw.SessionID,
		IssuedAt:     issuedAt,
		ExpiresAt:    expiresAt,
		AppMetadata:  raw.AppMetadata,
		UserMetadata: raw.UserMetadata,
	}, nil
}

func signature(unsigned string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func mustDecode(value string) []byte {
	decoded, _ := base64.RawURLEncoding.DecodeString(value)
	return decoded
}

func validationNow(options ValidationOptions) time.Time {
	if options.Now.IsZero() {
		return time.Now()
	}
	return options.Now
}

func parseUnixClaim(value json.Number, name string) (int64, error) {
	if value == "" {
		return 0, fmt.Errorf("%s is missing", name)
	}
	parsed, err := value.Int64()
	if err != nil {
		return 0, fmt.Errorf("%s is invalid", name)
	}
	return parsed, nil
}

func parseOptionalUnixClaim(value json.Number) (int64, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := value.Int64()
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
