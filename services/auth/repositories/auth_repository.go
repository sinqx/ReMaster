package repositories

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	models "remaster/services/auth/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TODO: refactor errors
const (
	UsersCollection         = "users"
	RefreshTokensCollection = "refresh_tokens"
	LoginAttemptsCollection = "login_attempts"
)

type AuthRepository struct {
	collection              *mongo.Collection
	refreshTokensCollection *mongo.Collection
	logger                  *slog.Logger
}

func NewAuthRepository(db *mongo.Database, logger *slog.Logger) *AuthRepository {
	return &AuthRepository{
		collection:              db.Collection("users"),
		refreshTokensCollection: db.Collection("refresh_tokens"),
		logger:                  logger.With(slog.String("repository", "auth")),
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
	IsUniqueConstraintError(err error) bool
}

func (r *AuthRepository) Create(ctx context.Context, user *models.User) error {
	user.BeforeCreate()

	count, err := r.collection.CountDocuments(ctx, bson.M{"email": user.Email})
	if err != nil {
		return fmt.Errorf("failed to check email uniqueness: %w", err)
	}

	if count > 0 {
		return fmt.Errorf("user with email %s already exists", user.Email)
	}

	_, err = r.collection.InsertOne(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *AuthRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}

func (r *AuthRepository) SaveRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	opts := options.Update().SetUpsert(true)
	filter := bson.M{"user_id": token.UserID}
	update := bson.M{"$set": token}

	_, err := r.refreshTokensCollection.UpdateOne(ctx, filter, update, opts)
	return err
}

func (r *AuthRepository) IsUniqueConstraintError(err error) bool {
	if writeException, ok := err.(mongo.WriteException); ok {
		for _, writeError := range writeException.WriteErrors {
			if writeError.Code == 11000 { // 11000 - duplication error code MongoDB
				return true
			}
		}
	}
	return false
}
