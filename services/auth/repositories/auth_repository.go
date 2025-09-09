package repositories

import (
	"context"
	"log/slog"
	"time"

	models "remaster/services/auth/models"

	"go.mongodb.org/mongo-driver/mongo"
)

const (
	UsersCollection         = "users"
	RefreshTokensCollection = "refresh_tokens"
	LoginAttemptsCollection = "login_attempts"
)

type AuthRepository struct {
	collection *mongo.Collection
	logger     *slog.Logger
}

func NewAuthRepository(db *mongo.Database, logger *slog.Logger) *AuthRepository {
	return &AuthRepository{
		collection: db.Collection("users"),
		logger:     logger,
	}
}

type AuthRepositoryInterface interface {
	// User operations
	CreateUser(ctx context.Context, user *models.User) error
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

func (r *AuthRepository) CreateUser(ctx context.Context, user *models.User) error {
	_, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		r.logger.Error("failed to create user in db", "error", err, "email", user.Email)
		return err
	}
	return nil
}
