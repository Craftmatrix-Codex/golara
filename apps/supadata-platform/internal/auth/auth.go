package auth

import (
	"crypto/subtle"
	"strings"
)

// HasValidBearerToken validates an exact bearer token without revealing token
// values through logs or error messages.
func HasValidBearerToken(configuredToken, authorization string) bool {
	if configuredToken == "" || !strings.HasPrefix(authorization, "Bearer ") {
		return false
	}
	provided := strings.TrimPrefix(authorization, "Bearer ")
	if provided == "" || len(provided) != len(configuredToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(configuredToken)) == 1
}
