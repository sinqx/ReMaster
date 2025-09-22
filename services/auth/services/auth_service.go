package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"remaster/services/auth/models"
	oauth "remaster/services/auth/oAuth"
	repo "remaster/services/auth/repositories"
	"remaster/services/auth/utils"
	conn "remaster/shared/connection"
	et "remaster/shared/errors"

	"github.com/cenkalti/backoff/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

const (
	MaxLoginAttempts = 5
	LockoutDuration  = 15 * time.Minute
	BcryptCost       = 12
)

type AuthService struct {
	repo       repo.AuthRepositoryInterface
	redisMgr   *conn.RedisManager
	googleAuth *oauth.GoogleAuthClient
	// oauthConfig *config.OAuthConfig
	jwtUtils *utils.JWTUtils
}

func NewAuthService(
	userRepo repo.AuthRepositoryInterface,
	redisMgr *conn.RedisManager,
	googleAuth *oauth.GoogleAuthClient,
	jwtUtils *utils.JWTUtils,

) *AuthService {
	return &AuthService{
		repo:       userRepo,
		redisMgr:   redisMgr,
		googleAuth: googleAuth,
		jwtUtils:   jwtUtils,
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
		var conflictErr *et.ConflictError
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
