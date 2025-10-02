package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	MaxLoginAttempts = 5
	LockoutDuration  = 15 * time.Minute
)

type RateLimiter struct {
	client *redis.Client
}

func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

// Check quantity of login attempts
func (rl *RateLimiter) CheckLoginAttempts(ctx context.Context, email string) (bool, int, error) {
	key := fmt.Sprintf("login:attempts:%s", email)

	count, err := rl.client.Get(ctx, key).Int()
	if err == redis.Nil {
		return true, 0, nil // first attempt
	}
	if err != nil {
		return false, 0, err
	}

	if count >= MaxLoginAttempts {
		ttl, _ := rl.client.TTL(ctx, key).Result()
		return false, count, fmt.Errorf("too many login attempts, try again in %s", ttl)
	}

	return true, count, nil
}

func (rl *RateLimiter) IncrementLoginAttempts(ctx context.Context, email string) error {
	key := fmt.Sprintf("login:attempts:%s", email)

	pipe := rl.client.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, LockoutDuration)

	_, err := pipe.Exec(ctx)
	return err
}

func (rl *RateLimiter) ResetLoginAttempts(ctx context.Context, email string) error {
	key := fmt.Sprintf("login:attempts:%s", email)
	return rl.client.Del(ctx, key).Err()
}
