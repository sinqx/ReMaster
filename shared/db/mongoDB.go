package db

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"remaster/shared/config"
)

// mongo collections (for now)
const (
	UsersCollection    = "users"
	MastersCollection  = "masters"
	OrdersCollection   = "orders"
	ReviewsCollection  = "reviews"
	ChatsCollection    = "chats"
	MessagesCollection = "messages"
	MediaCollection    = "media"
)

type MongoManager struct {
	client   *mongo.Client
	database *mongo.Database
	config   *config.MongoConfig
	mu       sync.RWMutex
}

var (
	mongoInstance *MongoManager
	mongoOnce     sync.Once
)

// Singleton
func NewMongoManager(cfg *config.MongoConfig) *MongoManager {
	mongoOnce.Do(func() {
		mongoInstance = &MongoManager{
			config: cfg,
		}
	})
	return mongoInstance
}

// create MongoDB connection
func (m *MongoManager) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client != nil {
		if err := m.client.Ping(ctx, readpref.Primary()); err == nil {
			log.Println("MongoDB connection already established")
			return nil
		}
		m.client.Disconnect(ctx)
	}

	connectCtx, cancel := context.WithTimeout(ctx, m.config.ConnectTimeout)
	defer cancel()

	// client settings
	clientOptions := options.Client().
		ApplyURI(m.config.URI).
		SetMaxPoolSize(m.config.MaxPoolSize).
		SetMinPoolSize(m.config.MinPoolSize).
		SetServerSelectionTimeout(m.config.ServerSelection).
		SetConnectTimeout(m.config.ConnectTimeout)

	// client creation
	client, err := mongo.Connect(connectCtx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to create MongoDB client: %w", err)
	}

	// check conn
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		client.Disconnect(ctx)
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	m.client = client
	m.database = client.Database(m.config.Database)

	log.Printf("Successfully connected to MongoDB database: %s", m.config.Database)
	return nil
}

// Disconnection from MongoDB
func (m *MongoManager) Disconnect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client == nil {
		return nil
	}

	if err := m.client.Disconnect(ctx); err != nil {
		return fmt.Errorf("failed to disconnect from MongoDB: %w", err)
	}

	m.client = nil
	m.database = nil
	log.Println("MongoDB connection closed")
	return nil
}

// Get database instance
func (m *MongoManager) GetDatabase() *mongo.Database {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.database
}

// Get mongo client
func (m *MongoManager) GetClient() *mongo.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client
}

// Get mongo collection by name
func (m *MongoManager) GetCollection(name string) *mongo.Collection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.database == nil {
		return nil
	}
	return m.database.Collection(name)
}

// MongoDB HealthCheck
func (m *MongoManager) HealthCheck(ctx context.Context) error {
	m.mu.RLock()
	client := m.client
	m.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("MongoDB client is not initialized")
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		return fmt.Errorf("MongoDB health check failed: %w", err)
	}

	return nil
}

// === DB HELPER FUNCTIONS ===

// mongo func with transaction
func (m *MongoManager) WithTransaction(ctx context.Context, fn func(sessCtx mongo.SessionContext) error) error {
	m.mu.RLock()
	client := m.client
	m.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("MongoDB client is not initialized")
	}

	session, err := client.StartSession()
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)

	callback := func(sessCtx mongo.SessionContext) (any, error) {
		return nil, fn(sessCtx)
	}

	_, err = session.WithTransaction(ctx, callback)
	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

// // create indexes
// func (m *MongoManager) CreateIndexes(ctx context.Context) error {
// 	if m.database == nil {
// 		return fmt.Errorf("database is not initialized")
// 	}
// 	indexes := map[string][]mongo.IndexModel{
// 		UsersCollection: {
// 			{
// 				Keys:    map[string]any{"email": 1},
// 				Options: options.Index().SetUnique(true),
// 			},
// 			{
// 				Keys: map[string]any{"user_type": 1},
// 			},
// 			{
// 				Keys: map[string]any{"status": 1},
// 			},
// 			{
// 				Keys: map[string]any{"created_at": -1},
// 			},
// 		},
// 		MastersCollection: {
// 			{
// 				Keys:    map[string]any{"user_id": 1},
// 				Options: options.Index().SetUnique(true),
// 			},
// 			{
// 				Keys: map[string]any{"specialization": 1},
// 			},
// 			{
// 				Keys: map[string]any{"rating": -1},
// 			},
// 			{
// 				Keys: map[string]any{"is_verified": 1},
// 			},
// 			{
// 				Keys: map[string]any{"location": "2dsphere"},
// 			},
// 		},
// 		OrdersCollection: {
// 			{
// 				Keys: map[string]any{"client_id": 1},
// 			},
// 			{
// 				Keys: map[string]any{"master_id": 1},
// 			},
// 			{
// 				Keys: map[string]any{"status": 1},
// 			},
// 			{
// 				Keys: map[string]any{"category": 1},
// 			},
// 			{
// 				Keys: map[string]any{"created_at": -1},
// 			},
// 			{
// 				Keys: map[string]any{"location": "2dsphere"},
// 			},
// 		},
// 		ReviewsCollection: {
// 			{
// 				Keys:    map[string]any{"order_id": 1, "reviewer_id": 1},
// 				Options: options.Index().SetUnique(true),
// 			},
// 			{
// 				Keys: map[string]any{"master_id": 1},
// 			},
// 			{
// 				Keys: map[string]any{"rating": 1},
// 			},
// 			{
// 				Keys: map[string]any{"created_at": -1},
// 			},
// 		},
// 		ChatsCollection: {
// 			{
// 				Keys: map[string]any{"participants": 1},
// 			},
// 			{
// 				Keys:    map[string]any{"order_id": 1},
// 				Options: options.Index().SetUnique(true).SetSparse(true),
// 			},
// 			{
// 				Keys: map[string]any{"last_message_at": -1},
// 			},
// 		},
// 		MessagesCollection: {
// 			{
// 				Keys: map[string]any{"chat_id": 1, "created_at": -1},
// 			},
// 			{
// 				Keys: map[string]any{"sender_id": 1},
// 			},
// 		},
// 		MediaCollection: {
// 			{
// 				Keys: map[string]any{"owner_id": 1},
// 			},
// 			{
// 				Keys: map[string]any{"entity_type": 1, "entity_id": 1},
// 			},
// 			{
// 				Keys: map[string]any{"created_at": -1},
// 			},
// 		},
// 	}
// 	for collectionName, indexModels := range indexes {
// 		collection := m.database.Collection(collectionName)
// 		if len(indexModels) > 0 {
// 			indexNames, err := collection.Indexes().CreateMany(ctx, indexModels)
// 			if err != nil {
// 				log.Printf("Failed to create indexes for %s: %v", collectionName, err)
// 				continue
// 			}
// 			log.Printf("Created %d indexes for collection %s: %v", len(indexNames), collectionName, indexNames)
// 		}
// 	}
// 	return nil
// }

// MongoDB Stats
func (m *MongoManager) Stats(ctx context.Context) (map[string]any, error) {
	if m.database == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	var result map[string]any
	err := m.database.RunCommand(ctx, map[string]any{"dbStats": 1}).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("failed to get database stats: %w", err)
	}

	return result, nil
}

// === COLLECTION HELPERS ===

func (m *MongoManager) GetUsersCollection() *mongo.Collection {
	return m.GetCollection(UsersCollection)
}
func (m *MongoManager) GetMastersCollection() *mongo.Collection {
	return m.GetCollection(MastersCollection)
}
func (m *MongoManager) GetOrdersCollection() *mongo.Collection {
	return m.GetCollection(OrdersCollection)
}
func (m *MongoManager) GetReviewsCollection() *mongo.Collection {
	return m.GetCollection(ReviewsCollection)
}
func (m *MongoManager) GetChatsCollection() *mongo.Collection {
	return m.GetCollection(ChatsCollection)
}
func (m *MongoManager) GetMessagesCollection() *mongo.Collection {
	return m.GetCollection(MessagesCollection)
}
func (m *MongoManager) GetMediaCollection() *mongo.Collection {
	return m.GetCollection(MediaCollection)
}
