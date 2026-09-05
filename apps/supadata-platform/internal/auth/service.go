package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/jwt"
)

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrUserAlreadyRegistered = errors.New("user already registered")
	ErrInvalidCredentials    = errors.New("invalid login credentials")
	ErrInvalidRefreshToken   = errors.New("invalid refresh token")
)

type User struct {
	ID               string         `json:"id"`
	Email            string         `json:"email"`
	Role             string         `json:"role"`
	AppMetadata      map[string]any `json:"app_metadata,omitempty"`
	UserMetadata     map[string]any `json:"user_metadata,omitempty"`
	CreatedAt        time.Time      `json:"created_at,omitempty"`
	UpdatedAt        time.Time      `json:"updated_at,omitempty"`
	EmailConfirmedAt *time.Time     `json:"email_confirmed_at,omitempty"`
}

type Session struct {
	ID               string
	UserID           string
	RefreshTokenHash string
	ExpiresAt        time.Time
}

type Repository interface {
	CreateUser(context.Context, string, string, map[string]any, bool) (User, error)
	FindUserByEmail(context.Context, string) (User, string, error)
	CreateSession(context.Context, string, string, time.Time) (Session, error)
	RefreshSession(context.Context, string, string, time.Time) (User, Session, error)
}

type SessionRevoker interface {
	RevokeSession(context.Context, string) error
}

type ServiceOptions struct {
	JWTSecret   []byte
	Issuer      string
	Audience    string
	TokenTTL    time.Duration
	AutoConfirm bool
	Now         func() time.Time
}

type Service struct {
	repository  Repository
	jwtSecret   []byte
	issuer      string
	audience    string
	tokenTTL    time.Duration
	autoConfirm bool
	now         func() time.Time
}

type SessionResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

func NewService(repository Repository, options ServiceOptions) *Service {
	ttl := options.TokenTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	audience := options.Audience
	if audience == "" {
		audience = "authenticated"
	}
	return &Service{
		repository:  repository,
		jwtSecret:   append([]byte(nil), options.JWTSecret...),
		issuer:      options.Issuer,
		audience:    audience,
		tokenTTL:    ttl,
		autoConfirm: options.AutoConfirm,
		now:         now,
	}
}

func HashPassword(password string) (string, error) {
	if len(password) < 6 {
		return "", errors.New("password should be at least 6 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func VerifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func (s *Service) SignUp(ctx context.Context, email, password string, metadata map[string]any) (SessionResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return SessionResponse{}, errors.New("invalid email")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return SessionResponse{}, err
	}
	user, err := s.repository.CreateUser(ctx, email, hash, metadata, s.autoConfirm)
	if errors.Is(err, ErrUserAlreadyRegistered) {
		return SessionResponse{}, ErrUserAlreadyRegistered
	}
	if err != nil {
		return SessionResponse{}, fmt.Errorf("create user: %w", err)
	}
	if !s.autoConfirm {
		return SessionResponse{User: user}, nil
	}
	return s.issueSession(ctx, user)
}

func (s *Service) SignIn(ctx context.Context, email, password string) (SessionResponse, error) {
	user, passwordHash, err := s.repository.FindUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return SessionResponse{}, ErrInvalidCredentials
	}
	if err := VerifyPassword(passwordHash, password); err != nil {
		return SessionResponse{}, ErrInvalidCredentials
	}
	return s.issueSession(ctx, user)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (SessionResponse, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return SessionResponse{}, ErrInvalidRefreshToken
	}
	if len(s.jwtSecret) == 0 {
		return SessionResponse{}, errors.New("JWT secret is required")
	}
	now := s.now().UTC()
	expiresAt := now.Add(s.tokenTTL)
	rotatedRefreshToken, err := randomToken(32)
	if err != nil {
		return SessionResponse{}, fmt.Errorf("generate refresh token: %w", err)
	}
	user, session, err := s.repository.RefreshSession(ctx, hashToken(refreshToken), hashToken(rotatedRefreshToken), expiresAt)
	if err != nil {
		return SessionResponse{}, ErrInvalidRefreshToken
	}
	accessToken, err := s.issueAccessToken(user, session.ID, now, expiresAt)
	if err != nil {
		return SessionResponse{}, err
	}
	return SessionResponse{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		ExpiresIn:    int64(s.tokenTTL.Seconds()),
		ExpiresAt:    expiresAt.Unix(),
		RefreshToken: rotatedRefreshToken,
		User:         user,
	}, nil
}

func (s *Service) Logout(ctx context.Context, accessToken string) error {
	if len(s.jwtSecret) == 0 || strings.TrimSpace(accessToken) == "" {
		return ErrInvalidCredentials
	}
	claims, err := jwt.VerifyHS256(accessToken, s.jwtSecret, jwt.ValidationOptions{Now: s.now(), Issuer: s.issuer, Audience: s.audience})
	if err != nil || claims.Subject == "" || claims.SessionID == "" {
		return ErrInvalidCredentials
	}
	revoker, ok := s.repository.(SessionRevoker)
	if !ok {
		return errors.New("session revocation is not configured")
	}
	if err := revoker.RevokeSession(ctx, claims.SessionID); err != nil {
		return fmt.Errorf("revoke auth session: %w", err)
	}
	return nil
}

func (s *Service) GetUserByAccessToken(ctx context.Context, accessToken string) (User, error) {
	if len(s.jwtSecret) == 0 || strings.TrimSpace(accessToken) == "" {
		return User{}, ErrInvalidCredentials
	}
	claims, err := jwt.VerifyHS256(accessToken, s.jwtSecret, jwt.ValidationOptions{Now: s.now(), Issuer: s.issuer, Audience: s.audience})
	if err != nil || claims.Subject == "" {
		return User{}, ErrInvalidCredentials
	}
	repository, ok := s.repository.(interface {
		FindUserByID(context.Context, string) (User, error)
	})
	if !ok {
		return User{}, errors.New("user lookup is not configured")
	}
	user, err := repository.FindUserByID(ctx, claims.Subject)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Service) issueSession(ctx context.Context, user User) (SessionResponse, error) {
	if len(s.jwtSecret) == 0 {
		return SessionResponse{}, errors.New("JWT secret is required")
	}
	now := s.now().UTC()
	expiresAt := now.Add(s.tokenTTL)
	refreshToken, err := randomToken(32)
	if err != nil {
		return SessionResponse{}, fmt.Errorf("generate refresh token: %w", err)
	}
	session, err := s.repository.CreateSession(ctx, user.ID, hashToken(refreshToken), expiresAt)
	if err != nil {
		return SessionResponse{}, fmt.Errorf("create session: %w", err)
	}
	accessToken, err := s.issueAccessToken(user, session.ID, now, expiresAt)
	if err != nil {
		return SessionResponse{}, err
	}
	return SessionResponse{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		ExpiresIn:    int64(s.tokenTTL.Seconds()),
		ExpiresAt:    expiresAt.Unix(),
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

func (s *Service) issueAccessToken(user User, sessionID string, now, expiresAt time.Time) (string, error) {
	accessToken, err := jwt.SignHS256(jwt.Claims{
		Subject:      user.ID,
		Email:        user.Email,
		Role:         user.Role,
		Audience:     s.audience,
		Issuer:       s.issuer,
		SessionID:    sessionID,
		IssuedAt:     now.Unix(),
		ExpiresAt:    expiresAt.Unix(),
		AppMetadata:  user.AppMetadata,
		UserMetadata: user.UserMetadata,
	}, s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("issue access token: %w", err)
	}
	return accessToken, nil
}

func randomToken(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func hashToken(value string) string {
	digest := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}
