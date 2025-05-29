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
		redisClient = redis.NewClient(&redis.Options{
			Addr:     os.Getenv("REDIS_ADD"),
			Password: os.Getenv("REDIS_PASS"),
			DB:       0,
		})

		if _, err := redisClient.Ping(Ctx).Result(); err != nil {
			log.Fatalf("❌ Failed to connect to Redis: %v", err)
		}
		log.Println("✅ Connected to Redis")
	})
	return redisClient
}
