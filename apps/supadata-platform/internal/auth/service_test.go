package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRepository struct {
	user          User
	passwordHash  string
	created       bool
	sessionCreate bool
	session       Session
}

func (r *fakeRepository) CreateUser(_ context.Context, email, passwordHash string, metadata map[string]any, confirmed bool) (User, error) {
	if r.created {
		return User{}, ErrUserAlreadyRegistered
	}
	r.created = true
	r.passwordHash = passwordHash
	r.user = User{ID: "user-1", Email: email, Role: "authenticated", UserMetadata: metadata}
	if confirmed {
		now := time.Now().UTC()
		r.user.EmailConfirmedAt = &now
	}
	return r.user, nil
}
func (r *fakeRepository) FindUserByEmail(context.Context, string) (User, string, error) {
	if !r.created {
		return User{}, "", ErrUserNotFound
	}
	return r.user, r.passwordHash, nil
}
func (r *fakeRepository) FindUserByID(context.Context, string) (User, error) {
	if !r.created {
		return User{}, ErrUserNotFound
	}
	return r.user, nil
}

func (r *fakeRepository) CreateSession(_ context.Context, userID, refreshTokenHash string, expiresAt time.Time) (Session, error) {
	r.sessionCreate = true
	r.session = Session{ID: "session-1", UserID: userID, RefreshTokenHash: refreshTokenHash, ExpiresAt: expiresAt}
	return r.session, nil
}
func (r *fakeRepository) RefreshSession(_ context.Context, oldHash, newHash string, expiresAt time.Time) (User, Session, error) {
	if oldHash != r.session.RefreshTokenHash {
		return User{}, Session{}, ErrInvalidRefreshToken
	}
	r.session.RefreshTokenHash = newHash
	r.session.ExpiresAt = expiresAt
	return r.user, r.session, nil
}

func TestPasswordHashDoesNotStorePlaintextAndVerifies(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password was stored as plaintext")
	}
	if err := VerifyPassword(hash, "correct horse battery staple"); err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if err := VerifyPassword(hash, "wrong password"); err == nil {
		t.Fatal("wrong password was accepted")
	}
}

func TestSignUpAndPasswordSignInIssueSupabaseSessionShape(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository, ServiceOptions{
		JWTSecret:   []byte("auth-test-secret"),
		Issuer:      "https://example.invalid/auth/v1",
		Audience:    "authenticated",
		TokenTTL:    time.Hour,
		AutoConfirm: true,
	})

	signedUp, err := service.SignUp(context.Background(), "user@example.com", "password-123456", map[string]any{"name": "User"})
	if err != nil {
		t.Fatalf("SignUp() error = %v", err)
	}
	if signedUp.User.ID != "user-1" || signedUp.AccessToken == "" || signedUp.RefreshToken == "" || signedUp.TokenType != "bearer" {
		t.Fatalf("unexpected signup response: %+v", signedUp)
	}
	if !repository.sessionCreate {
		t.Fatal("signup did not create a session")
	}

	signedIn, err := service.SignIn(context.Background(), "user@example.com", "password-123456")
	if err != nil {
		t.Fatalf("SignIn() error = %v", err)
	}
	if signedIn.User.ID != "user-1" || signedIn.AccessToken == "" || signedIn.RefreshToken == "" {
		t.Fatalf("unexpected signin response: %+v", signedIn)
	}
}

func TestRefreshRotatesRefreshTokenAndReturnsNewAccessToken(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository, ServiceOptions{JWTSecret: []byte("auth-test-secret"), TokenTTL: time.Hour, AutoConfirm: true})
	initial, err := service.SignUp(context.Background(), "user@example.com", "password-123456", nil)
	if err != nil {
		t.Fatalf("SignUp() error = %v", err)
	}
	refreshed, err := service.Refresh(context.Background(), initial.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" || refreshed.RefreshToken == initial.RefreshToken {
		t.Fatalf("refresh token was not rotated: %+v", refreshed)
	}
}

func TestGetUserByAccessTokenValidatesJWTAndReadsRepository(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository, ServiceOptions{JWTSecret: []byte("auth-test-secret"), TokenTTL: time.Hour, AutoConfirm: true})
	issued, err := service.SignUp(context.Background(), "user@example.com", "password-123456", nil)
	if err != nil {
		t.Fatalf("SignUp() error = %v", err)
	}
	user, err := service.GetUserByAccessToken(context.Background(), issued.AccessToken)
	if err != nil {
		t.Fatalf("GetUserByAccessToken() error = %v", err)
	}
	if user.ID != "user-1" || user.Email != "user@example.com" {
		t.Fatalf("user = %+v, want repository user", user)
	}
}

func TestSignInDoesNotRevealWhetherEmailExists(t *testing.T) {
	service := NewService(&fakeRepository{}, ServiceOptions{JWTSecret: []byte("auth-test-secret"), TokenTTL: time.Hour})
	_, err := service.SignIn(context.Background(), "missing@example.com", "password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("SignIn() error = %v, want invalid credentials", err)
	}
}
