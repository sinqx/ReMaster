package utils

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	config "remaster/shared"

	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	UserType string `json:"user_type"`
	jwt.RegisteredClaims
}

type JWTUtils struct {
	secretKey       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func NewJWTUtils(jwtConfig *config.JWTConfig) *JWTUtils {
	return &JWTUtils{
		secretKey:       jwtConfig.SecretKey,
		AccessTokenTTL:  jwtConfig.AccessTokenTTL,
		RefreshTokenTTL: jwtConfig.RefreshTokenTTL,
	}
}

func (j *JWTUtils) GenerateAccessToken(userID, email, userType string) (string, error) {
	claims := CustomClaims{
		UserID:   userID,
		Email:    email,
		UserType: userType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secretKey))
}

func (j *JWTUtils) GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
