package auth

import "testing"

func TestBearerTokenRequiresExactConfiguredToken(t *testing.T) {
	tests := []struct {
		name          string
		configured    string
		authorization string
		want          bool
	}{
		{name: "missing configured token", configured: "", authorization: "Bearer secret", want: false},
		{name: "missing header", configured: "secret", authorization: "", want: false},
		{name: "wrong scheme", configured: "secret", authorization: "Basic secret", want: false},
		{name: "wrong token", configured: "secret", authorization: "Bearer wrong", want: false},
		{name: "exact token", configured: "secret", authorization: "Bearer secret", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasValidBearerToken(tt.configured, tt.authorization); got != tt.want {
				t.Fatalf("HasValidBearerToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
