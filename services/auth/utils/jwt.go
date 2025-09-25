package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	config "remaster/shared"

	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	UserID    string          `json:"user_id"`
	Email     string          `json:"email"`
	UserType  string          `json:"user_type"`
	ExpiresAt jwt.NumericDate `json:"expires_at"`
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

func (j *JWTUtils) ParseRefreshToken(token string) (*CustomClaims, error) {
	tokenClaims, err := jwt.ParseWithClaims(token, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(j.secretKey), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := tokenClaims.Claims.(*CustomClaims); ok && tokenClaims.Valid {
		return claims, nil
	}
	return nil, err
}

func (j *JWTUtils) ValidateAccessToken(tokenStr string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(j.secretKey), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}
