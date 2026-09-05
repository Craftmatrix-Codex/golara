package jwt

import (
	"testing"
	"time"
)

func BenchmarkSignHS256(b *testing.B) {
	claims := Claims{Subject: "00000000-0000-4000-8000-000000000001", Role: "authenticated", Audience: "authenticated", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	secret := []byte("benchmark-secret")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := SignHS256(claims, secret); err != nil {
			b.Fatal(err)
		}
	}
}
