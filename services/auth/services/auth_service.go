package services

import (
	"context"
	"fmt"
	"time"

	"remaster/services/auth/models"
	oauth "remaster/services/auth/oAuth"
	"remaster/services/auth/repositories"
	"remaster/services/auth/utils"
	config "remaster/shared"
	conn "remaster/shared/connection"

	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

const (
	MaxLoginAttempts = 5
	LockoutDuration  = 15 * time.Minute
	BcryptCost       = 12
)

type AuthService struct {
	repo        repositories.AuthRepository
	redisMgr    *conn.RedisManager
	googleAuth  *oauth.GoogleAuthClient
	oauthConfig *config.OAuthConfig
	jwtConfig   *config.JWTConfig
	jwtUtils    *utils.JWTUtils
}

func NewAuthService(
	userRepo repositories.AuthRepository,
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
	Register(ctx context.Context, req *models.RegisterRequest) (string, error)
}

func (s *AuthService) Register(ctx context.Context, req *models.RegisterRequest, metadata *models.RequestMetadata) (*models.AuthResponse, error) {
	// s.logger.Info("Registering user", "email", req.Email)

	if err := req.ValidateRegisterRequest(); err != nil {
		return nil, err // NewValidationError(err.Error())
	}

	existingUser, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err // fmt.Errorf("error checking user existence: %w", err)
	}
	if existingUser != nil {
		return nil, err //NewConflictError("user with this email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		Email:     req.Email,
		Password:  string(hashedPassword),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		UserType:  req.UserType,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		// if s.repo.IsUniqueConstraintError(err)
		return nil, fmt.Errorf("failed to create user: %w", err)
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
		ExpiresAt: time.Now().Add(s.jwtConfig.RefreshTokenTTL),
		CreatedAt: time.Now(),
		IsRevoked: false, // Новый токен не может быть отозван
		DeviceID:  metadata.DeviceID,
		UserAgent: metadata.UserAgent,
		IP:        metadata.IPAddress,
	}

	if err := s.repo.SaveRefreshToken(ctx, tokenModel); err != nil {
		return nil, fmt.Errorf("failed to save refresh token: %w", err)
	}

	// s.recordLoginAttempt(ctx, req.Email, metadata.IPAddress, true, "registration")
	// s.logger.Info("User registered successfully", "userID", user.ID.Hex())

	return &models.AuthResponse{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.jwtConfig.AccessTokenTTL).Unix(),
		TokenType:    "Bearer",
	}, nil
}
