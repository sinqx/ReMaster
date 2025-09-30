package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"remaster/services/auth/models"
	oauth "remaster/services/auth/oauth"
	repo "remaster/services/auth/repositories"
	"remaster/services/auth/utils"
	conn "remaster/shared/connection"
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
	redisMgr     *conn.RedisManager
	oauthFactory *oauth.ProviderFactory
	jwtUtils *utils.JWTUtils
}

func NewAuthService(
	userRepo repo.AuthRepositoryInterface,
	redisMgr *conn.RedisManager,
	oauthFactory *oauth.ProviderFactory,
	jwtUtils *utils.JWTUtils,

) *AuthService {
	return &AuthService{
		repo:         userRepo,
		redisMgr:     redisMgr,
		oauthFactory: oauthFactory,
		jwtUtils:     jwtUtils,
	}
}

type AuthServiceInterface interface {
	CreateUser(ctx context.Context, req *models.RegisterRequest) (string, error)
}

func (s *AuthService) CreateUser(ctx context.Context, req *models.RegisterRequest, metadata *models.RequestMetadata) (*models.AuthResponse, error) {
	if err := req.ValidateRegisterRequest(); err != nil {
		return nil, et.NewValidationError("failed to validate register request",
			map[string]string{"error": err.Error()}) // TODO: refactor after validation lib import
	}

	existingUser, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, et.NewDatabaseError("failed to check existing user", err)
	}
	if existingUser != nil {
		return nil, et.NewConflictError("user with this email already exists", nil)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), BcryptCost)
	if err != nil {
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
			return nil, err
		}
		return nil, fmt.Errorf("service: %w", err)
	}

	accessToken, err := s.jwtUtils.GenerateAccessToken(user.ID.Hex(), user.Email, string(user.UserType))
	if err != nil {
		return nil, err
	}
	refreshToken, err := s.jwtUtils.GenerateRefreshToken()
	if err != nil {
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
	user, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, et.NewUnauthorizedError("invalid email or password")
		}
		return nil, et.NewDatabaseError("failed to fetch user", err)
	}

	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		return nil, et.NewPermissionError("account is locked, please try again later")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		attempts, _ := s.repo.IncrementLoginAttempts(ctx, user.ID)
		if attempts >= MaxLoginAttempts {
			_ = s.repo.LockUserAccount(ctx, user.ID, LockoutDuration)
		}
		return nil, et.NewUnauthorizedError("invalid email or password")
	}

	if user.LoginAttempts > 0 {
		_ = s.repo.ResetLoginAttempts(ctx, user.ID)
	}

	_ = s.repo.UpdateLoginInfo(ctx, user.ID, metadata.IPAddress)

	accessToken, err := s.jwtUtils.GenerateAccessToken(user.ID.Hex(), user.Email, string(user.UserType))
	if err != nil {
		return nil, et.NewInternalError("failed to generate access token", err)
	}
	refreshToken, err := s.jwtUtils.GenerateRefreshToken()
	if err != nil {
		return nil, et.NewInternalError("failed to generate refresh token", err)
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
		return nil, et.NewDatabaseError("failed to save refresh token", err)
	}

	return &models.AuthResponse{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.jwtUtils.AccessTokenTTL).Unix(),
		TokenType:    "Bearer",
	}, nil
}

func (s *AuthService) OAuthLogin(ctx context.Context, req *models.OAuthLoginRequest, metadata *models.RequestMetadata) (*models.AuthResponse, error) {
	provider, err := s.oauthFactory.GetProvider(oauth.ProviderType(req.Provider))
	if err != nil {
		return nil, et.NewInternalError("invalid oauth provider", err)
	}

	claims, err := provider.VerifyIDToken(ctx, req.IDToken)
	if err != nil {
		return nil, et.NewUnauthorizedError(err.Error())
	}

	user, err := s.repo.GetByEmail(ctx, claims.Email)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, et.NewDatabaseError("failed to find user", err)
	}

	if user == nil {
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
			return nil, et.NewDatabaseError("failed to create user", err)
		}
	} else {
		if user.LoginAttempts > 0 {
			_ = s.repo.ResetLoginAttempts(ctx, user.ID)
		}

		_ = s.repo.UpdateLoginInfo(ctx, user.ID, metadata.IPAddress)
	}

	accessToken, err := s.jwtUtils.GenerateAccessToken(user.ID.Hex(), user.Email, string(user.UserType))
	if err != nil {
		return nil, err
	}
	refreshToken, err := s.jwtUtils.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	if err := s.repo.SaveRefreshToken(ctx, &models.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(s.jwtUtils.RefreshTokenTTL),
		CreatedAt: time.Now(),
	}); err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.jwtUtils.AccessTokenTTL).Unix(),
		TokenType:    "Bearer",
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, req *models.RefreshTokenRequest, metadata *models.RequestMetadata) (*models.RefreshTokenResponse, error) {
	storedToken, err := s.repo.FindRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}

	if storedToken.IsRevoked {
		return nil, et.NewUnauthorizedError("refresh token has been revoked")
	}
	if time.Now().After(storedToken.ExpiresAt) {
		return nil, et.NewUnauthorizedError("refresh token has expired")
	}

	if err := s.repo.RevokeRefreshToken(ctx, storedToken.ID); err != nil {
		return nil, err
	}

	user, err := s.repo.GetByID(ctx, storedToken.UserID)
	if err != nil {
		return nil, et.NewNotFoundError("user associated with token not found", err)
	}

	accessToken, err := s.jwtUtils.GenerateAccessToken(user.ID.Hex(), user.Email, string(user.UserType))
	if err != nil {
		return nil, et.NewInternalError("failed to generate access token", err)
	}
	newRefreshToken, err := s.jwtUtils.GenerateRefreshToken()
	if err != nil {
		return nil, et.NewInternalError("failed to generate refresh token", err)
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
		return nil, et.NewDatabaseError("failed to save new refresh token", err)
	}

	return &models.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    time.Now().Add(s.jwtUtils.AccessTokenTTL).Unix(),
	}, nil
}

func (s *AuthService) ValidateToken(ctx context.Context, req *models.ValidateTokenRequest) (*models.ValidateTokenResponse, error) {
	claims, err := s.jwtUtils.ValidateAccessToken(req.AccessToken)
	if err != nil {
		return nil, et.NewUnauthorizedError("invalid or expired token")
	}

	userID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return nil, et.NewInternalError("failed to parse user id", err)
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, et.NewDatabaseError("failed to fetch user", err)
	}

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
	userID, err := primitive.ObjectIDFromHex(req.UserId)
	if err != nil {
		return et.NewValidationError("invalid user id", map[string]string{"user_id": req.UserId})
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return et.NewDatabaseError("failed to fetch user", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return et.NewUnauthorizedError("old password is incorrect")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), BcryptCost)
	if err != nil {
		return et.NewInternalError("failed to hash new password", err)
	}

	err = s.repo.UpdatePassword(ctx, user.ID, string(hashedPassword))
	if err != nil {
		return et.NewDatabaseError("failed to update password", err)
	}
	return nil
}

func (s *AuthService) Logout(ctx context.Context, req *models.LogoutRequest) error {
	tokenID, err := primitive.ObjectIDFromHex(req.RefreshToken)
	if err != nil {
		return et.NewValidationError("invalid refresh token id", map[string]string{"refresh_token": req.RefreshToken})
	}
	return s.repo.RevokeRefreshToken(ctx, tokenID)
}
