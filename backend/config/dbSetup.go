package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	UserCollection           *mongo.Collection
	PropertyCollection       *mongo.Collection
	FavoriteCollection       *mongo.Collection
	RecommendationCollection *mongo.Collection
)

func ConnectDB() (*mongo.Client, error) {
	MONGO_URI := os.Getenv("MONGOURI")
	if MONGO_URI == "" {
		return nil, fmt.Errorf("MONGO_URI not set in environment")
	}

	clientOptions := options.Client().ApplyURI(MONGO_URI)
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("MongoDB ping failed: %v", err)
	}

	log.Println("Connected to MongoDB")
	return client, nil
}

func InitCollections(client *mongo.Client) {
	dbName := os.Getenv("DB")
	UserCollection = client.Database(dbName).Collection("users")
	PropertyCollection = client.Database(dbName).Collection("properties")
	FavoriteCollection = client.Database(dbName).Collection("favorites")
	RecommendationCollection = client.Database(dbName).Collection("recommendations")
}

func CloseDBConnection(client *mongo.Client) {
	if err := client.Disconnect(context.TODO()); err != nil {
		log.Fatalf("Error closing database connection: %v", err)
	}
	log.Println("MongoDB connection closed")
}
