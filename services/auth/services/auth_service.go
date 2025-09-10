package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"strings"
	"time"

	"remaster/services/auth/models"
	oauth "remaster/services/auth/oAuth"
	"remaster/services/auth/repositories"
	"remaster/services/auth/utils"
	config "remaster/shared"
	conn "remaster/shared/connection"

	"golang.org/x/crypto/bcrypt"
)

const (
	MaxLoginAttempts = 5
	LockoutDuration  = 15 * time.Minute
	BcryptCost       = 12
)

type AuthService struct {
	userRepo    repositories.AuthRepository
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
		userRepo:   userRepo,
		redisMgr:   redisMgr,
		googleAuth: googleAuth,
		jwtUtils:   jwtUtils,
	}
}

type AuthServiceInterface interface {
	Register(ctx context.Context, req *models.CreateUserRequest) (string, error)
}

func (s *AuthService) Register(ctx context.Context, req *models.CreateUserRequest) (*models.AuthResponse, error) {
	// logger.Info("Registering user: %s", req.Email)

	if err := s.validateCreateUserRequest(req); err != nil {
		return nil, err // NewValidationError(err.Error())
	}

	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		return nil, err // NewConflictError("user already exists")
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

	if err := s.userRepo.Create(ctx, user); err != nil {
		// if s.userRepo.IsUniqueConstraintError(err) {
		// 	return nil, err // NewConflictError("user already exists")
		// }
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	accessToken, err := s.jwtUtils.GenerateAccessToken(user.ID.Hex(), user.Email, string(user.UserType))
	if err != nil {
		return nil, err //fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtUtils.GenerateRefreshToken()
	if err != nil {
		return nil, err // fmt.Errorf("failed to generate refresh token: %w", err)
	}

	tokenModel := &models.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(s.jwtConfig.RefreshTokenTTL),
	}

	if err := s.userRepo.SaveRefreshToken(ctx, tokenModel); err != nil {
		return nil, err // fmt.Errorf("failed to save refresh token: %w", err)
	}

	// s.recordLoginAttempt(ctx, req.Email, "", true, "registration")

	log.Printf("User registered successfully: %s", user.ID.Hex())

	return &models.AuthResponse{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(s.jwtConfig.AccessTokenTTL).Unix(),
		TokenType:    "Bearer",
	}, nil
}

func (s *AuthService) validateCreateUserRequest(req *models.CreateUserRequest) error {
	var errs []string

	if _, err := mail.ParseAddress(req.Email); err != nil {
		errs = append(errs, "invalid email format")
	}
	if len(req.Password) < 8 {
		errs = append(errs, "password must be at least 8 characters long")
	}
	if strings.TrimSpace(req.FirstName) == "" {
		errs = append(errs, "first name is required")
	}
	if strings.TrimSpace(req.LastName) == "" {
		errs = append(errs, "last name is required")
	}
	if req.UserType != models.UserTypeMaster && req.UserType != models.UserTypeClient {
		errs = append(errs, "invalid user type")
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}
