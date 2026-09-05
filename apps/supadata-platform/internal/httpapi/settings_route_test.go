package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthSettingsReturnsGoTrueCompatibleFields(t *testing.T) {
	server := NewServer(ServerOptions{Token: "secret", Registry: &fakeRegistry{}, APIKeys: APIKeyConfig{Anon: "anon-key"}, AuthSettings: AuthSettings{EmailEnabled: true, MailerAutoconfirm: true}})
	request := httptest.NewRequest(http.MethodGet, "/auth/v1/settings", nil)
	request.Header.Set("apikey", "anon-key")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("settings status = %d, want 200", response.Code)
	}
	var body struct {
		External struct {
			Email bool `json:"email"`
		} `json:"external"`
		MailerAutoconfirm bool `json:"mailer_autoconfirm"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if !body.External.Email || !body.MailerAutoconfirm {
		t.Fatalf("settings body = %#v, want enabled email and autoconfirm", body)
	}
}
