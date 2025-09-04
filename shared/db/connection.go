package db

import (
	"context"
	"fmt"
	"log"
	"remaster/shared/config"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	UserCollection = "users"
)

var (
	clientInstance *mongo.Client
	once           sync.Once
)

func NewMongoClient(ctx context.Context, cfg *config.MongoConfig) (*mongo.Client, error) {
	if cfg.URI == "" {
		return nil, fmt.Errorf("mongodb URI is required")
	}
	if cfg.DB == "" {
		return nil, fmt.Errorf("mongodb database is required")
	}

	var err error
	once.Do(func() {
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()

		client, connErr := mongo.Connect(cctx, options.Client().ApplyURI(cfg.URI))
		if connErr != nil {
			err = fmt.Errorf("failed to connect to MongoDB: %w", connErr)
			return
		}

		if pingErr := client.Ping(cctx, readpref.Primary()); pingErr != nil {
			err = fmt.Errorf("failed to ping MongoDB: %w", pingErr)
			return
		}

		log.Printf("✅ Successfully connected to MongoDB at %s", cfg.URI)
		clientInstance = client
	})
	return clientInstance, err
}

// GetDatabase returns a database instance
func GetDatabase(client *mongo.Client, cfg *config.MongoConfig) *mongo.Database {
	return client.Database(cfg.DB)
}
