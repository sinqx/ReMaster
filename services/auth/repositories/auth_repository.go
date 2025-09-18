package repositories

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	models "remaster/services/auth/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type authRepositoryImpl struct {
	usersCol         *mongo.Collection
	refreshTokensCol *mongo.Collection
	loginAttemptsCol *mongo.Collection
	logger           *slog.Logger
}

func NewAuthRepository(db *mongo.Database, logger *slog.Logger) *authRepositoryImpl {
	return &authRepositoryImpl{
		usersCol:         db.Collection("users"),
		refreshTokensCol: db.Collection("refresh_tokens"),
		loginAttemptsCol: db.Collection("login_attempts"),
		logger:           logger.With(slog.String("repository", "auth")),
	}
}

type AuthRepositoryInterface interface {
	// User operations
	Create(ctx context.Context, user *models.User) error
	// GetByID(ctx context.Context, id string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)

	// Refresh token operations
	SaveRefreshToken(ctx context.Context, token *models.RefreshToken) error
	// GetRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error)
	// RevokeRefreshToken(ctx context.Context, token string) error
	// RevokeAllUserRefreshTokens(ctx context.Context, userID string) error
	// CleanExpiredTokens(ctx context.Context) error

	// Login attempts
	// SaveLoginAttempt(ctx context.Context, attempt *models.LoginAttempt) error
	// GetLoginAttempts(ctx context.Context, email string, since time.Time) ([]*models.LoginAttempt, error)

	// Utility
	EnsureIndexes(ctx context.Context) error
	IsUniqueConstraintError(err error) bool
}

func (r *authRepositoryImpl) EnsureIndexes(ctx context.Context) error {
	// unique index on email
	_, err := r.usersCol.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("idx_users_email_unique"),
	})
	if err != nil {
		return fmt.Errorf("create users.email index: %w", err)
	}

	// add other indexes if needed
	return nil
}

func (r *authRepositoryImpl) Create(ctx context.Context, user *models.User) error {
	user.BeforeCreate()

	_, err := r.usersCol.InsertOne(ctx, user)
	if err != nil {
		if r.IsUniqueConstraintError(err) {
			return fmt.Errorf("user with email %s already exists: %w", user.Email, err)
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (r *authRepositoryImpl) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := r.usersCol.FindOne(ctx, bson.M{"email": email}).Decode(&u)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, mongo.ErrNoDocuments
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &u, nil
}

func (r *authRepositoryImpl) SaveRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	opts := options.Update().SetUpsert(true)
	filter := bson.M{"user_id": token.UserID}
	update := bson.M{"$set": token}

	_, err := r.refreshTokensCol.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save refresh token: %w", err)
	}
	return nil
}

func (r *authRepositoryImpl) IsUniqueConstraintError(err error) bool {
	var we mongo.WriteException
	if errors.As(err, &we) {
		for _, e := range we.WriteErrors {
			// 11000 is duplicate key
			if e.Code == 11000 {
				return true
			}
		}
	}
	// some drivers may return CommandError
	var ce mongo.CommandError
	if errors.As(err, &ce) {
		if ce.Code == 11000 {
			return true
		}
	}
	return false
}
