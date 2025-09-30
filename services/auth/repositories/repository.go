package repositories

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	models "remaster/services/auth/models"
	et "remaster/shared/errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error)
	UpdateLoginInfo(ctx context.Context, userID primitive.ObjectID, ipAddress string) error
	LockUserAccount(ctx context.Context, userID primitive.ObjectID, duration time.Duration) error
	UpdatePassword(ctx context.Context, userID primitive.ObjectID, hashedPassword string) error

	// Refresh token operations
	SaveRefreshToken(ctx context.Context, token *models.RefreshToken) error
	FindRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, token primitive.ObjectID) error

	// Login attempts
	IncrementLoginAttempts(ctx context.Context, userID primitive.ObjectID) (int, error)
	ResetLoginAttempts(ctx context.Context, userID primitive.ObjectID) error

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

	return nil
}

func (r *authRepositoryImpl) Create(ctx context.Context, user *models.User) error {
	user.BeforeCreate()

	_, err := r.usersCol.InsertOne(ctx, user)
	if err != nil {
		if r.IsUniqueConstraintError(err) {
			return et.NewConflictError(fmt.Sprintf("user with email %s already exists", user.Email), err)
		}
		return et.NewDatabaseError("failed to create user", err)
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

func (r *authRepositoryImpl) GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	var u models.User
	err := r.usersCol.FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, mongo.ErrNoDocuments
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return &u, nil
}

func (r *authRepositoryImpl) SaveRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	token.ID = primitive.NewObjectID()
	_, err := r.refreshTokensCol.InsertOne(ctx, token)
	if err != nil {
		return fmt.Errorf("failed to save refresh token: %w", err)
	}
	return nil
}

func (r *authRepositoryImpl) FindRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error) {
	var rt models.RefreshToken
	err := r.refreshTokensCol.FindOne(ctx, bson.M{"token": token}).Decode(&rt)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, et.NewUnauthorizedError("refresh token not found")
		}
		return nil, et.NewDatabaseError("failed to find refresh token", err)
	}
	return &rt, nil
}

func (r *authRepositoryImpl) RevokeRefreshToken(ctx context.Context, tokenID primitive.ObjectID) error {
	filter := bson.M{"_id": tokenID}
	update := bson.M{"$set": bson.M{"is_revoked": true}}
	_, err := r.refreshTokensCol.UpdateOne(ctx, filter, update)
	if err != nil {
		return et.NewDatabaseError("failed to revoke refresh token", err)
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

func (r *authRepositoryImpl) UpdateLoginInfo(ctx context.Context, userID primitive.ObjectID, ipAddress string) error {
	update := bson.M{
		"$set": bson.M{
			"last_login_at": time.Now(),
			"last_login_ip": ipAddress,
		},
	}
	_, err := r.usersCol.UpdateByID(ctx, userID, update)
	return err
}

func (r *authRepositoryImpl) LockUserAccount(ctx context.Context, userID primitive.ObjectID, duration time.Duration) error {
	lockedUntil := time.Now().Add(duration)
	update := bson.M{"$set": bson.M{"locked_until": lockedUntil}}
	_, err := r.usersCol.UpdateByID(ctx, userID, update)
	return err
}

func (r *authRepositoryImpl) UpdatePassword(ctx context.Context, userID primitive.ObjectID, hashedPassword string) error {
	filter := bson.M{"_id": userID}
	update := bson.M{"$set": bson.M{"password": hashedPassword}}
	_, err := r.usersCol.UpdateOne(ctx, filter, update)
	if err != nil {
		return et.NewDatabaseError("failed to update password", err)
	}
	return nil
}

func (r *authRepositoryImpl) IncrementLoginAttempts(ctx context.Context, userID primitive.ObjectID) (int, error) {
	update := bson.M{"$inc": bson.M{"login_attempts": 1}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updatedUser models.User
	err := r.usersCol.FindOneAndUpdate(ctx, bson.M{"_id": userID}, update, opts).Decode(&updatedUser)
	if err != nil {
		return 0, err
	}
	return updatedUser.LoginAttempts, nil
}

func (r *authRepositoryImpl) ResetLoginAttempts(ctx context.Context, userID primitive.ObjectID) error {
	update := bson.M{"$set": bson.M{"login_attempts": 0}, "$unset": bson.M{"locked_until": ""}}
	_, err := r.usersCol.UpdateByID(ctx, userID, update)
	return err
}
