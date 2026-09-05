package jwt

import (
	"strings"
	"testing"
	"time"
)

func TestHS256TokenRoundTripAndClaimsValidation(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	token, err := SignHS256(Claims{
		Subject:   "user-id",
		Role:      "authenticated",
		Audience:  "authenticated",
		Issuer:    "https://example.invalid/auth/v1",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(time.Hour).Unix(),
	}, []byte("jwt-test-secret"))
	if err != nil {
		t.Fatalf("SignHS256() error = %v", err)
	}

	claims, err := VerifyHS256(token, []byte("jwt-test-secret"), ValidationOptions{
		Now:      now,
		Audience: "authenticated",
		Issuer:   "https://example.invalid/auth/v1",
	})
	if err != nil {
		t.Fatalf("VerifyHS256() error = %v", err)
	}
	if claims.Subject != "user-id" || claims.Role != "authenticated" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestVerifyHS256RejectsTamperingWrongSecretAlgorithmAndExpiry(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	token, err := SignHS256(Claims{Subject: "user-id", ExpiresAt: now.Add(time.Minute).Unix()}, []byte("jwt-test-secret"))
	if err != nil {
		t.Fatalf("SignHS256() error = %v", err)
	}
	for _, name := range []string{"tampered", "wrong secret"} {
		candidate := token
		secret := []byte("jwt-test-secret")
		if name == "tampered" {
			candidate = strings.TrimSuffix(token, "a") + "b"
		} else {
			secret = []byte("wrong-secret")
		}
		if _, err := VerifyHS256(candidate, secret, ValidationOptions{Now: now}); err == nil {
			t.Errorf("VerifyHS256(%s) accepted invalid token", name)
		}
	}
	if _, err := VerifyHS256(token, []byte("jwt-test-secret"), ValidationOptions{Now: now.Add(2 * time.Minute)}); err == nil {
		t.Error("VerifyHS256 accepted expired token")
	}
	if _, err := VerifyHS256(strings.Replace(token, "eyJhbGciOiJIUzI1NiIs", "eyJhbGciOiJub25lIiIs", 1), []byte("jwt-test-secret"), ValidationOptions{Now: now}); err == nil {
		t.Error("VerifyHS256 accepted a non-HS256 algorithm")
	}
}
