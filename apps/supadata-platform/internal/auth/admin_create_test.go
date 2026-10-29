package auth

import (
	"context"
	"testing"
)

func TestAdminCreatesConfirmedUserWithoutSigningIn(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository, ServiceOptions{JWTSecret: []byte("secret")})
	user, err := service.AdminCreateUser(context.Background(), " SDK@EXAMPLE.INVALID ", "password-123", map[string]any{"fixture": true}, true)
	if err != nil || user.Email != "sdk@example.invalid" || user.EmailConfirmedAt == nil {
		t.Fatalf("user=%+v err=%v", user, err)
	}
	if repository.sessionCreate {
		t.Fatal("admin create must not create a user session")
	}
	if VerifyPassword(repository.passwordHash, "password-123") != nil {
		t.Fatal("password was not securely hashed")
	}
}

func TestUnconfirmedUserCannotSignInBeforeConfirmation(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository, ServiceOptions{JWTSecret: []byte("secret")})
	if _, err := service.SignUp(context.Background(), "sdk@example.invalid", "password-123", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SignIn(context.Background(), "sdk@example.invalid", "password-123"); err == nil {
		t.Fatal("unconfirmed signup created a password session")
	}
	if repository.sessionCreate {
		t.Fatal("unconfirmed user must not receive a session")
	}
}
