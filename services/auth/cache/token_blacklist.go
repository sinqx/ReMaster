package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type TokenBlacklist struct {
	client *redis.Client
}

func NewTokenBlacklist(client *redis.Client) *TokenBlacklist {
	return &TokenBlacklist{client: client}
}

func (tb *TokenBlacklist) AddToBlacklist(ctx context.Context, tokenJTI string, expiresAt time.Time) error {
	key := fmt.Sprintf("blacklist:token:%s", tokenJTI)
	ttl := time.Until(expiresAt)

	if ttl <= 0 {
		return nil // token expired
	}

	return tb.client.Set(ctx, key, "1", ttl).Err()
}

func (tb *TokenBlacklist) IsBlacklisted(ctx context.Context, tokenJTI string) (bool, error) {
	key := fmt.Sprintf("blacklist:token:%s", tokenJTI)

	exists, err := tb.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return exists > 0, nil
}
