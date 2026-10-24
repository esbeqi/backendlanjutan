package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"api-students/config"
)

// NewMongo connects to MongoDB, verifies connectivity with Ping, and returns
// the configured database handle. The caller owns the client lifecycle.
func NewMongo(ctx context.Context) (*mongo.Client, *mongo.Database, error) {
	uri := strings.TrimSpace(config.GetEnv("MONGO_URI", ""))
	databaseName := strings.TrimSpace(config.GetEnv("MONGO_DATABASE", ""))
	if uri == "" {
		return nil, nil, fmt.Errorf("MONGO_URI wajib diatur")
	}
	if databaseName == "" {
		return nil, nil, fmt.Errorf("MONGO_DATABASE wajib diatur")
	}

	client, err := mongo.Connect(
		options.Client().
			ApplyURI(uri).
			SetServerSelectionTimeout(5 * time.Second),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("konfigurasi MongoDB tidak valid: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		if disconnectErr := client.Disconnect(disconnectCtx); disconnectErr != nil {
			return nil, nil, fmt.Errorf("gagal terhubung ke MongoDB: %w (gagal membersihkan client: %v)", err, disconnectErr)
		}
		return nil, nil, fmt.Errorf("gagal terhubung ke MongoDB: %w", err)
	}

	return client, client.Database(databaseName), nil
}
