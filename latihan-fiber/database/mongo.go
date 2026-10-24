package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"latihan-fiber/config"
)

// NewMongo membuat client MongoDB dan memastikan server dapat dihubungi.
func NewMongo(ctx context.Context) (*mongo.Client, *mongo.Database, error) {
	clientOptions := options.Client().
		ApplyURI(config.GetEnv("MONGO_URI", "mongodb://localhost:27017")).
		SetServerSelectionTimeout(5 * time.Second)

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("gagal membuat client MongoDB: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if disconnectErr := client.Disconnect(cleanupCtx); disconnectErr != nil {
			return nil, nil, fmt.Errorf("gagal terhubung ke MongoDB: %w (gagal membersihkan client: %v)", err, disconnectErr)
		}
		return nil, nil, fmt.Errorf("gagal terhubung ke MongoDB: %w", err)
	}

	return client, client.Database(config.GetEnv("MONGO_DATABASE", "praktikum_backend")), nil
}
