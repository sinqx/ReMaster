package repositories

import (
	"context"
	"time"

	models "remaster/services/auth/models"
)

const (
	UsersCollection         = "users"
	RefreshTokensCollection = "refresh_tokens"
	LoginAttemptsCollection = "login_attempts"
)

type UserRepository interface {
	// User operations
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByGoogleID(ctx context.Context, googleID string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	UpdatePassword(ctx context.Context, userID, hashedPassword string) error
	UpdateLoginAttempts(ctx context.Context, userID string, attempts int) error
	UpdateLastLogin(ctx context.Context, userID, ip string) error

	// Refresh token operations
	SaveRefreshToken(ctx context.Context, token *models.RefreshToken) error
	GetRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, token string) error
	RevokeAllUserRefreshTokens(ctx context.Context, userID string) error
	CleanExpiredTokens(ctx context.Context) error

	// Login attempts
	SaveLoginAttempt(ctx context.Context, attempt *models.LoginAttempt) error
	GetLoginAttempts(ctx context.Context, email string, since time.Time) ([]*models.LoginAttempt, error)

	// Utility
	CreateIndexes(ctx context.Context) error
}
