package connection

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	cfg "remaster/shared"

	"github.com/redis/go-redis/v9"
)

type RedisManager struct {
	client *redis.Client
	config *cfg.RedisConfig
	mu     sync.RWMutex
}

var (
	redisInstance *RedisManager
	redisOnce     sync.Once
)

// singleton
func NewRedisManager(cfg *cfg.RedisConfig) *RedisManager {
	redisOnce.Do(func() {
		redisInstance = &RedisManager{config: cfg}
	})
	return redisInstance
}

func (r *RedisManager) Connect(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.client != nil {
		if err := r.client.Ping(ctx).Err(); err == nil {
			log.Println("Redis connection already established")
			return nil
		}
		_ = r.client.Close()
		r.client = nil
	}

	r.client = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", r.config.Host, r.config.Port),
		Password:     r.config.Password,
		DB:           r.config.DB,
		PoolSize:     r.config.PoolSize,
		MinIdleConns: r.config.MinIdleConns,
		DialTimeout:  r.config.DialTimeout,
		ReadTimeout:  r.config.ReadTimeout,
		WriteTimeout: r.config.WriteTimeout,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.client.Ping(pingCtx).Err(); err != nil {
		_ = r.client.Close()
		r.client = nil
		return fmt.Errorf("failed to ping Redis: %w", err)
	}

	log.Println("Successfully connected to Redis")
	return nil
}

func (r *RedisManager) Disconnect() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.client == nil {
		return nil
	}
	if err := r.client.Close(); err != nil {
		return fmt.Errorf("failed to disconnect Redis: %w", err)
	}
	r.client = nil
	log.Println("Redis connection closed")
	return nil
}

func (r *RedisManager) HealthCheck(ctx context.Context) error {
	r.mu.RLock()
	client := r.client
	r.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return client.Ping(ctx).Err()
}

func (r *RedisManager) GetClient() *redis.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client
}

// === HELPER FUNCTIONS ===
func (r *RedisManager) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}
func (r *RedisManager) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}
func (r *RedisManager) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}
func (r *RedisManager) Exists(ctx context.Context, key string) (bool, error) {
	res, err := r.client.Exists(ctx, key).Result()
	return res > 0, err
}
func (r *RedisManager) Stats(ctx context.Context) (map[string]any, error) {
	info, err := r.client.Info(ctx, "stats").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis stats: %w", err)
	}
	return map[string]any{"info": info}, nil
}
