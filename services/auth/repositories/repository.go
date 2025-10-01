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
	repo := &authRepositoryImpl{
		usersCol:         db.Collection("users"),
		refreshTokensCol: db.Collection("refresh_tokens"),
		loginAttemptsCol: db.Collection("login_attempts"),
		logger:           logger.With(slog.String("auth", "repository")),
	}
	return repo
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
	RevokeRefreshToken(ctx context.Context, tokenID primitive.ObjectID) error

	// Login attempts
	IncrementLoginAttempts(ctx context.Context, userID primitive.ObjectID) (int, error)
	ResetLoginAttempts(ctx context.Context, userID primitive.ObjectID) error

	// Utility
	EnsureIndexes(ctx context.Context) error
	IsUniqueConstraintError(err error) bool
}

func (r *authRepositoryImpl) EnsureIndexes(ctx context.Context) error {
	r.logger.Info("Creating database indexes")

	// unique index on email
	_, err := r.usersCol.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("idx_users_email_unique"),
	})
	if err != nil {
		r.logger.Error("Failed to create users.email index", "error", err)
		return fmt.Errorf("create users.email index: %w", err)
	}

	r.logger.Info("Database indexes created successfully")
	return nil
}

func (r *authRepositoryImpl) Create(ctx context.Context, user *models.User) error {
	r.logger.Info("Creating new user", "email", user.Email)

	user.BeforeCreate()

	_, err := r.usersCol.InsertOne(ctx, user)
	if err != nil {
		if r.IsUniqueConstraintError(err) {
			r.logger.Warn("Unique constraint violation", "email", user.Email, "error", err)
			return et.NewConflictError(fmt.Sprintf("user with email %s already exists", user.Email), err)
		}
		r.logger.Error("Failed to insert user", "error", err)
		return et.NewDatabaseError("failed to create user", err)
	}

	r.logger.Info("User created successfully", "user_id", user.ID.Hex())
	return nil
}

func (r *authRepositoryImpl) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	r.logger.Info("Fetching user by email", "email", email)

	var u models.User
	err := r.usersCol.FindOne(ctx, bson.M{"email": email}).Decode(&u)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			r.logger.Warn("User not found by email", "email", email)
			return nil, mongo.ErrNoDocuments
		}
		r.logger.Error("Failed to fetch user by email", "error", err)
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	r.logger.Info("User fetched successfully", "user_id", u.ID.Hex())
	return &u, nil
}

func (r *authRepositoryImpl) GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	r.logger.Info("Fetching user by ID", "user_id", id.Hex())

	var u models.User
	err := r.usersCol.FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			r.logger.Warn("User not found by ID", "user_id", id.Hex())
			return nil, mongo.ErrNoDocuments
		}
		r.logger.Error("Failed to fetch user by ID", "error", err)
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	r.logger.Info("User fetched successfully", "user_id", u.ID.Hex())
	return &u, nil
}

func (r *authRepositoryImpl) SaveRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	r.logger.Info("Saving refresh token", "user_id", token.UserID.Hex())

	token.ID = primitive.NewObjectID()
	_, err := r.refreshTokensCol.InsertOne(ctx, token)
	if err != nil {
		r.logger.Error("Failed to save refresh token", "error", err)
		return fmt.Errorf("failed to save refresh token: %w", err)
	}

	r.logger.Info("Refresh token saved successfully", "token_id", token.ID.Hex())
	return nil
}

func (r *authRepositoryImpl) FindRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error) {
	r.logger.Info("Finding refresh token")

	var rt models.RefreshToken
	err := r.refreshTokensCol.FindOne(ctx, bson.M{"token": token}).Decode(&rt)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			r.logger.Warn("Refresh token not found")
			return nil, et.NewUnauthorizedError("refresh token not found")
		}
		r.logger.Error("Failed to find refresh token", "error", err)
		return nil, et.NewDatabaseError("failed to find refresh token", err)
	}

	r.logger.Info("Refresh token found", "token_id", rt.ID.Hex())
	return &rt, nil
}

func (r *authRepositoryImpl) RevokeRefreshToken(ctx context.Context, tokenID primitive.ObjectID) error {
	r.logger.Info("Revoking refresh token", "token_id", tokenID.Hex())

	filter := bson.M{"_id": tokenID}
	update := bson.M{"$set": bson.M{"is_revoked": true}}
	_, err := r.refreshTokensCol.UpdateOne(ctx, filter, update)
	if err != nil {
		r.logger.Error("Failed to revoke refresh token", "error", err)
		return et.NewDatabaseError("failed to revoke refresh token", err)
	}

	r.logger.Info("Refresh token revoked successfully", "token_id", tokenID.Hex())
	return nil
}

func (r *authRepositoryImpl) IsUniqueConstraintError(err error) bool {
	r.logger.Debug("Checking if error is unique constraint violation")

	var we mongo.WriteException
	if errors.As(err, &we) {
		for _, e := range we.WriteErrors {
			// 11000 is duplicate key
			if e.Code == 11000 {
				r.logger.Debug("Unique constraint error detected")
				return true
			}
		}
	}
	// some drivers may return CommandError
	var ce mongo.CommandError
	if errors.As(err, &ce) {
		if ce.Code == 11000 {
			r.logger.Debug("Unique constraint error detected")
			return true
		}
	}
	r.logger.Debug("No unique constraint error")
	return false
}

func (r *authRepositoryImpl) UpdateLoginInfo(ctx context.Context, userID primitive.ObjectID, ipAddress string) error {
	r.logger.Info("Updating login info", "user_id", userID.Hex())

	update := bson.M{
		"$set": bson.M{
			"last_login_at": time.Now(),
			"last_login_ip": ipAddress,
		},
	}
	_, err := r.usersCol.UpdateByID(ctx, userID, update)
	if err != nil {
		r.logger.Error("Failed to update login info", "error", err)
		return err
	}

	r.logger.Info("Login info updated successfully", "user_id", userID.Hex())
	return nil
}

func (r *authRepositoryImpl) LockUserAccount(ctx context.Context, userID primitive.ObjectID, duration time.Duration) error {
	r.logger.Info("Locking user account", "user_id", userID.Hex(), "duration", duration)

	lockedUntil := time.Now().Add(duration)
	update := bson.M{"$set": bson.M{"locked_until": lockedUntil}}
	_, err := r.usersCol.UpdateByID(ctx, userID, update)
	if err != nil {
		r.logger.Error("Failed to lock user account", "error", err)
		return err
	}

	r.logger.Info("User account locked successfully", "user_id", userID.Hex())
	return nil
}

func (r *authRepositoryImpl) UpdatePassword(ctx context.Context, userID primitive.ObjectID, hashedPassword string) error {
	r.logger.Info("Updating password", "user_id", userID.Hex())

	filter := bson.M{"_id": userID}
	update := bson.M{"$set": bson.M{"password": hashedPassword}}
	_, err := r.usersCol.UpdateOne(ctx, filter, update)
	if err != nil {
		r.logger.Error("Failed to update password", "error", err)
		return et.NewDatabaseError("failed to update password", err)
	}

	r.logger.Info("Password updated successfully", "user_id", userID.Hex())
	return nil
}

func (r *authRepositoryImpl) IncrementLoginAttempts(ctx context.Context, userID primitive.ObjectID) (int, error) {
	r.logger.Info("Incrementing login attempts", "user_id", userID.Hex())

	update := bson.M{"$inc": bson.M{"login_attempts": 1}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updatedUser models.User
	err := r.usersCol.FindOneAndUpdate(ctx, bson.M{"_id": userID}, update, opts).Decode(&updatedUser)
	if err != nil {
		r.logger.Error("Failed to increment login attempts", "error", err)
		return 0, err
	}

	r.logger.Info("Login attempts incremented", "user_id", userID.Hex(), "attempts", updatedUser.LoginAttempts)
	return updatedUser.LoginAttempts, nil
}

func (r *authRepositoryImpl) ResetLoginAttempts(ctx context.Context, userID primitive.ObjectID) error {
	r.logger.Info("Resetting login attempts", "user_id", userID.Hex())

	update := bson.M{"$set": bson.M{"login_attempts": 0}, "$unset": bson.M{"locked_until": ""}}
	_, err := r.usersCol.UpdateByID(ctx, userID, update)
	if err != nil {
		r.logger.Error("Failed to reset login attempts", "error", err)
		return err
	}

	r.logger.Info("Login attempts reset successfully", "user_id", userID.Hex())
	return nil
}