package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"remaster/services/auth/models"
	oauth "remaster/services/auth/oauth"
	repo "remaster/services/auth/repositories"
	"remaster/services/auth/utils"
	et "remaster/shared/errors"

	"github.com/cenkalti/backoff/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

const (
	MaxLoginAttempts = 5
	LockoutDuration  = 15 * time.Minute
	BcryptCost       = 12
)

type AuthService struct {
	repo         repo.AuthRepositoryInterface
	oauthFactory *oauth.ProviderFactory
	jwtUtils     *utils.JWTUtils
	logger       *slog.Logger
}

func NewAuthService(
	userRepo repo.AuthRepositoryInterface,
	oauthFactory *oauth.ProviderFactory,
	jwtUtils *utils.JWTUtils,
	logger *slog.Logger,
) *AuthService {
	return &AuthService{
		repo:         userRepo,
		oauthFactory: oauthFactory,
		jwtUtils:     jwtUtils,
		logger:       logger.With(slog.String("auth", "service")),
	}
}

type AuthServiceInterface interface {
	CreateUser(ctx context.Context, req *models.RegisterRequest) (string, error)
}

func (s *AuthService) CreateUser(ctx context.Context, req *models.RegisterRequest, metadata *models.RequestMetadata) (*models.AuthResponse, error) {
	s.logger.Info("Starting user creation", "email", req.Email)

	if err := req.ValidateRegisterRequest(); err != nil {
		s.logger.Warn("Validation failed for registration", "error", err)
		return nil, et.NewValidationError("failed to validate register request",
			map[string]string{"error": err.Error()}) // TODO: refactor after validation lib import
	}

	existingUser, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		s.logger.Error("Failed to check existing user", "error", err)
		return nil, et.NewDatabaseError("failed to check existing user", err)
	}
	if existingUser != nil {
		s.logger.Warn("User already exists", "email", req.Email)
		return nil, et.NewConflictError("user with this email already exists", nil)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), BcryptCost)
	if err != nil {
		s.logger.Error("Failed to hash password", "error", err)
		return nil, et.NewInternalError("failed to hash password", err)
	}

	user := &models.User{
		Email:     req.Email,
		Password:  string(hashedPassword),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		UserType:  req.UserType,
	}

	err = backoff.Retry(func() error {
		return s.repo.Create(ctx, user)
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3))
	if err != nil {
		conflictErr := et.NewConflictError("", nil)
		if errors.As(err, &conflictErr) {
			s.logger.Warn("Conflict during user creation", "error", err)
			return nil, err
		}
		s.logger.Error("Failed to create user with retry", "error", err)
		return nil, fmt.Errorf("service: %w", err)
	}

	accessToken, err := s.jwtUtils.GenerateAccessToken(user.ID.Hex(), user.Email, string(user.UserType))
	if err != nil {
		s.logger.Error("Failed to generate access token", "error", err)
		return nil, err
	}
	refreshToken, err := s.jwtUtils.GenerateRefreshToken()
	if err != nil {
		s.logger.Error("Failed to generate refresh token", "error", err)
		return nil, err
	}

	tokenModel := &models.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(s.jwtUtils.RefreshTokenTTL),
		CreatedAt: time.Now(),
		IsRevoked: false,
		DeviceID:  metadata.DeviceID,
		UserAgent: metadata.UserAgent,
		IP:        metadata.IPAddress,
	}

	if err := s.repo.SaveRefreshToken(ctx, tokenModel); err != nil {
		s.logger.Error("Failed to save refresh token", "error", err)
		return nil, fmt.Errorf("failed to save refresh token: %w", err)
	}


	return &models.AuthResponse{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.jwtUtils.AccessTokenTTL).Unix(),
		TokenType:    "Bearer",
	}, nil
}

func (s *AuthService) AuthenticateUser(ctx context.Context, req *models.LoginRequest, metadata *models.RequestMetadata) (*models.AuthResponse, error) {
	s.logger.Info("Authenticating user", "email", req.Email)

	user, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			s.logger.Warn("Authentication failed: user not found", "email", req.Email)
			return nil, et.NewUnauthorizedError("invalid email or password")
		}
		s.logger.Error("Failed to fetch user for authentication", "error", err)
		return nil, et.NewDatabaseError("failed to fetch user", err)
	}

	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		s.logger.Warn("Account locked", "user_id", user.ID.Hex())
		return nil, et.NewPermissionError("account is locked, please try again later")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		attempts, _ := s.repo.IncrementLoginAttempts(ctx, user.ID)
		s.logger.Warn("Invalid password", "user_id", user.ID.Hex(), "attempts", attempts)
		if attempts >= MaxLoginAttempts {
			s.logger.Warn("Max attempts reached, locking account", "user_id", user.ID.Hex())
			_ = s.repo.LockUserAccount(ctx, user.ID, LockoutDuration)
		}
		return nil, et.NewUnauthorizedError("invalid email or password")
	}

	if user.LoginAttempts > 0 {
		s.logger.Info("Resetting login attempts", "user_id", user.ID.Hex())
		_ = s.repo.ResetLoginAttempts(ctx, user.ID)
	}

	s.logger.Info("Updating login info", "user_id", user.ID.Hex())
	_ = s.repo.UpdateLoginInfo(ctx, user.ID, metadata.IPAddress)

	accessToken, err := s.jwtUtils.GenerateAccessToken(user.ID.Hex(), user.Email, string(user.UserType))
	if err != nil {
		s.logger.Error("Failed to generate access token", "error", err)
		return nil, err
	}
	refreshToken, err := s.jwtUtils.GenerateRefreshToken()
	if err != nil {
		s.logger.Error("Failed to generate refresh token", "error", err)
		return nil, err
	}

	tokenModel := &models.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(s.jwtUtils.RefreshTokenTTL),
		CreatedAt: time.Now(),
		IsRevoked: false,
		DeviceID:  metadata.DeviceID,
		UserAgent: metadata.UserAgent,
		IP:        metadata.IPAddress,
	}

	if err := s.repo.SaveRefreshToken(ctx, tokenModel); err != nil {
		s.logger.Error("Failed to save refresh token", "error", err)
		return nil, et.NewDatabaseError("failed to save refresh token", err)
	}

	s.logger.Info("User authenticated successfully", "user_id", user.ID.Hex())
	return &models.AuthResponse{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.jwtUtils.AccessTokenTTL).Unix(),
		TokenType:    "Bearer",
	}, nil
}

func (s *AuthService) OAuthLogin(ctx context.Context, req *models.OAuthLoginRequest, metadata *models.RequestMetadata) (*models.AuthResponse, error) {
	s.logger.Info("Starting OAuth login", "provider", req.Provider)

	provider, err := s.oauthFactory.GetProvider(oauth.ProviderType(req.Provider))
	if err != nil {
		s.logger.Error("Invalid OAuth provider", "error", err)
		return nil, et.NewInternalError("invalid oauth provider", err)
	}

	claims, err := provider.VerifyIDToken(ctx, req.IDToken)
	if err != nil {
		s.logger.Error("Failed to verify ID token", "error", err)
		return nil, et.NewUnauthorizedError(err.Error())
	}

	user, err := s.repo.GetByEmail(ctx, claims.Email)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		s.logger.Error("Failed to find user for OAuth", "error", err)
		return nil, et.NewDatabaseError("failed to find user", err)
	}

	if user == nil {
		s.logger.Info("Creating new user from OAuth", "email", claims.Email)
		user = &models.User{
			Email:        claims.Email,
			FirstName:    claims.FirstName,
			LastName:     claims.LastName,
			ProfileImage: claims.Picture,
			UserType:     models.UserTypeClient,
			IsVerified:   true,
			IsActive:     true,
		}

		if err := s.repo.Create(ctx, user); err != nil {
			s.logger.Error("Failed to create OAuth user", "error", err)
			return nil, et.NewDatabaseError("failed to create user", err)
		}
	} else {
		s.logger.Info("OAuth user found, updating login", "user_id", user.ID.Hex())
		if user.LoginAttempts > 0 {
			_ = s.repo.ResetLoginAttempts(ctx, user.ID)
		}

		_ = s.repo.UpdateLoginInfo(ctx, user.ID, metadata.IPAddress)
	}

	accessToken, err := s.jwtUtils.GenerateAccessToken(user.ID.Hex(), user.Email, string(user.UserType))
	if err != nil {
		s.logger.Error("Failed to generate access token for OAuth", "error", err)
		return nil, err
	}
	refreshToken, err := s.jwtUtils.GenerateRefreshToken()
	if err != nil {
		s.logger.Error("Failed to generate refresh token for OAuth", "error", err)
		return nil, err
	}

	if err := s.repo.SaveRefreshToken(ctx, &models.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(s.jwtUtils.RefreshTokenTTL),
		CreatedAt: time.Now(),
	}); err != nil {
		s.logger.Error("Failed to save refresh token for OAuth", "error", err)
		return nil, err
	}

	s.logger.Info("OAuth login successful", "user_id", user.ID.Hex())
	return &models.AuthResponse{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.jwtUtils.AccessTokenTTL).Unix(),
		TokenType:    "Bearer",
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, req *models.RefreshTokenRequest, metadata *models.RequestMetadata) (*models.RefreshTokenResponse, error) {
	s.logger.Info("Refreshing token")

	storedToken, err := s.repo.FindRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		s.logger.Error("Failed to find stored refresh token", "error", err)
		return nil, err
	}

	if storedToken.IsRevoked {
		s.logger.Warn("Refresh token revoked", "token_id", storedToken.ID.Hex())
		return nil, et.NewUnauthorizedError("refresh token has been revoked")
	}
	if time.Now().After(storedToken.ExpiresAt) {
		s.logger.Warn("Refresh token expired", "token_id", storedToken.ID.Hex())
		return nil, et.NewUnauthorizedError("refresh token has expired")
	}

	if err := s.repo.RevokeRefreshToken(ctx, storedToken.ID); err != nil {
		s.logger.Error("Failed to revoke old refresh token", "error", err)
		return nil, err
	}

	user, err := s.repo.GetByID(ctx, storedToken.UserID)
	if err != nil {
		s.logger.Error("Failed to fetch user for token refresh", "error", err)
		return nil, et.NewNotFoundError("user associated with token not found", err)
	}

	accessToken, err := s.jwtUtils.GenerateAccessToken(user.ID.Hex(), user.Email, string(user.UserType))
	if err != nil {
		s.logger.Error("Failed to generate new access token", "error", err)
		return nil, err
	}
	newRefreshToken, err := s.jwtUtils.GenerateRefreshToken()
	if err != nil {
		s.logger.Error("Failed to generate new refresh token", "error", err)
		return nil, err
	}

	newTokenModel := &models.RefreshToken{
		UserID:    user.ID,
		Token:     newRefreshToken,
		ExpiresAt: time.Now().Add(s.jwtUtils.RefreshTokenTTL),
		CreatedAt: time.Now(),
		DeviceID:  metadata.DeviceID,
		UserAgent: metadata.UserAgent,
		IP:        metadata.IPAddress,
	}
	if err := s.repo.SaveRefreshToken(ctx, newTokenModel); err != nil {
		s.logger.Error("Failed to save new refresh token", "error", err)
		return nil, et.NewDatabaseError("failed to save new refresh token", err)
	}

	s.logger.Info("Token refreshed successfully", "user_id", user.ID.Hex())
	return &models.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    time.Now().Add(s.jwtUtils.AccessTokenTTL).Unix(),
	}, nil
}

func (s *AuthService) ValidateToken(ctx context.Context, req *models.ValidateTokenRequest) (*models.ValidateTokenResponse, error) {
	s.logger.Info("Validating access token")

	claims, err := s.jwtUtils.ValidateAccessToken(req.AccessToken)
	if err != nil {
		s.logger.Warn("Token validation failed", "error", err)
		return nil, et.NewUnauthorizedError("invalid or expired token")
	}

	userID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		s.logger.Error("Failed to parse user ID from claims", "error", err)
		return nil, et.NewInternalError("failed to parse user id", err)
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to fetch user for token validation", "error", err)
		return nil, et.NewDatabaseError("failed to fetch user", err)
	}

	s.logger.Info("Token validated successfully", "user_id", userID.Hex())
	return &models.ValidateTokenResponse{
		Valid:      true,
		UserID:     user.ID.Hex(),
		UserType:   user.UserType,
		IsActive:   user.IsActive,
		IsVerified: user.IsVerified,
		ExpiresAt:  claims.ExpiresAt.Time.Unix(),
	}, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, req *models.ChangePasswordRequest) error {
	s.logger.Info("Changing password", "user_id", req.UserID)

	userID, err := primitive.ObjectIDFromHex(req.UserID)
	if err != nil {
		s.logger.Warn("Invalid user ID", "user_id", req.UserID, "error", err)
		return et.NewValidationError("invalid user id", map[string]string{"user_id": req.UserID})
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to fetch user for password change", "error", err)
		return et.NewDatabaseError("failed to fetch user", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		s.logger.Warn("Old password mismatch", "user_id", userID.Hex())
		return et.NewUnauthorizedError("old password is incorrect")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), BcryptCost)
	if err != nil {
		s.logger.Error("Failed to hash new password", "error", err)
		return et.NewInternalError("failed to hash new password", err)
	}

	err = s.repo.UpdatePassword(ctx, user.ID, string(hashedPassword))
	if err != nil {
		s.logger.Error("Failed to update password in DB", "error", err)
		return et.NewDatabaseError("failed to update password", err)
	}

	s.logger.Info("Password changed successfully", "user_id", userID.Hex())
	return nil
}

func (s *AuthService) Logout(ctx context.Context, req *models.LogoutRequest) error {
	s.logger.Info("Logging out user", "user_id", req.UserID)

	tokenID, err := primitive.ObjectIDFromHex(req.RefreshToken)
	if err != nil {
		s.logger.Warn("Invalid refresh token ID", "refresh_token", req.RefreshToken, "error", err)
		return et.NewValidationError("invalid refresh token id", map[string]string{"refresh_token": req.RefreshToken})
	}

	err = s.repo.RevokeRefreshToken(ctx, tokenID)
	if err != nil {
		s.logger.Error("Failed to revoke token during logout", "error", err)
		return err
	}

	s.logger.Info("Logout successful", "user_id", req.UserID)
	return nil
}
