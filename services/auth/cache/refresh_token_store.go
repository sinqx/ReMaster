package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RefreshTokenData struct {
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type RefreshTokenStore struct {
	client *redis.Client
}

func NewRefreshTokenStore(client *redis.Client) *RefreshTokenStore {
	return &RefreshTokenStore{client: client}
}

func (rts *RefreshTokenStore) SaveRefreshToken(ctx context.Context, userID primitive.ObjectID, token string, expiresAt time.Time) error {
	key := fmt.Sprintf("refresh:token:%s", token)

	data := RefreshTokenData{
		UserID:    userID.Hex(),
		Token:     token,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	ttl := time.Until(expiresAt)
	return rts.client.Set(ctx, key, jsonData, ttl).Err()
}

func (rts *RefreshTokenStore) FindRefreshToken(ctx context.Context, token string) (*RefreshTokenData, error) {
	key := fmt.Sprintf("refresh:token:%s", token)

	data, err := rts.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("refresh token not found or expired")
	}
	if err != nil {
		return nil, err
	}

	var tokenData RefreshTokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return nil, err
	}

	return &tokenData, nil
}

func (rts *RefreshTokenStore) RevokeRefreshToken(ctx context.Context, token string) error {
	key := fmt.Sprintf("refresh:token:%s", token)
	return rts.client.Del(ctx, key).Err()
}

func (rts *RefreshTokenStore) RevokeAllUserTokens(ctx context.Context, userID primitive.ObjectID) error {
	pattern := "refresh:token:*"

	iter := rts.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()

		data, err := rts.client.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}

		var tokenData RefreshTokenData
		if err := json.Unmarshal(data, &tokenData); err != nil {
			continue
		}

		if tokenData.UserID == userID.Hex() {
			_ = rts.client.Del(ctx, key)
		}
	}

	return iter.Err()
}
