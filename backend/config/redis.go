package config

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/redis/go-redis/v9"
)

var (
	redisClient *redis.Client
	redisOnce   sync.Once
	Ctx         = context.Background()
)

func InitRedis() *redis.Client {
	redisOnce.Do(func() {
		redisURL := os.Getenv("REDIS_URL")
		if redisURL == "" {
			log.Fatal("❌ REDIS_URL is not set")
		}

		options, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Fatalf("❌ Failed to parse REDIS_URL: %v", err)
		}

		redisClient = redis.NewClient(options)

		if _, err := redisClient.Ping(Ctx).Result(); err != nil {
			log.Fatalf("❌ Failed to connect to Redis: %v", err)
		}
		log.Println("✅ Connected to Redis")
	})

	return redisClient
}
