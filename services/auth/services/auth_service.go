package services

import (
	"context"
	"time"

	"remaster/services/auth/models"
	"remaster/services/auth/repositories"
	config "remaster/shared"
	conn "remaster/shared/connection"
)

const (
	MaxLoginAttempts = 5
	LockoutDuration  = 15 * time.Minute
	BcryptCost       = 12
)

type AuthService struct {
	userRepo repositories.AuthRepository
	redisMgr *conn.RedisManager
	// googleAuth  *utils.GoogleAuth
	oauthConfig *config.OAuthConfig
	jwtConfig   *config.JWTConfig
	// jwtUtils    *utils.JWTUtils
}

func NewAuthService(
	userRepo repositories.AuthRepository,
	redisMgr *conn.RedisManager,
	oauthConfig *config.OAuthConfig,
	jwtConfig *config.JWTConfig,
) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		redisMgr: redisMgr,
		// googleAuth:  utils.NewGoogleAuth(oauthConfig),
		oauthConfig: oauthConfig,
		jwtConfig:   jwtConfig,
		// jwtUtils:    utils.NewJWTUtils(jwtConfig),
	}
}

type AuthServiceInterface interface {
	Register(ctx context.Context, req *models.CreateUserRequest) (string, error)
}
