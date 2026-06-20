package db

import (
	"context"
	"fmt"
	"log"

	"github.com/lijuuu/ChallengeWssManagerService/internal/config"
	"github.com/redis/go-redis/v9"
)

func NewRedisClient(cfg config.Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisURL,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	// Test connection
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Configure RDB persistence with more frequent saves for real-time data
	// Save if at least 1 key changed in 900 seconds (15 minutes)
	// Save if at least 10 keys changed in 300 seconds (5 minutes)
	// Save if at least 10000 keys changed in 60 seconds (1 minute)
	err = rdb.ConfigSet(ctx, "save", "900 1 300 10 60 10000").Err()
	if err != nil {
		log.Printf("Warning: Failed to set Redis RDB save configuration: %v", err)
	}

	// Set RDB filename for persistence
	err = rdb.ConfigSet(ctx, "dbfilename", "challenge-service.rdb").Err()
	if err != nil {
		log.Printf("Warning: Failed to set Redis RDB filename: %v", err)
	}

	fmt.Println("Connected to Redis successfully with RDB persistence configured")
	return rdb
}

// SaveRedisData forces a synchronous save of Redis data to RDB file
func SaveRedisData(rdb *redis.Client) error {
	ctx := context.Background()

	log.Println("Saving Redis data to RDB file...")
	err := rdb.BgSave(ctx).Err()
	if err != nil {
		// If background save fails, try synchronous save
		log.Printf("Background save failed, attempting synchronous save: %v", err)
		err = rdb.Save(ctx).Err()
		if err != nil {
			return fmt.Errorf("failed to save Redis data: %v", err)
		}
	}

	log.Println("Redis data saved successfully")
	return nil
}

// LoadRedisData checks if RDB file exists and loads data on startup
func LoadRedisData(rdb *redis.Client) error {
	ctx := context.Background()

	// Check if we have any data (RDB file would have been loaded automatically by Redis)
	info, err := rdb.Info(ctx, "persistence").Result()
	if err != nil {
		log.Printf("Warning: Could not get Redis persistence info: %v", err)
		return nil
	}

	log.Printf("Redis persistence info: %s", info)

	// Check if we have any keys loaded
	keyCount, err := rdb.DBSize(ctx).Result()
	if err != nil {
		log.Printf("Warning: Could not get Redis DB size: %v", err)
		return nil
	}

	if keyCount > 0 {
		log.Printf("Loaded %d keys from Redis RDB file", keyCount)
	} else {
		log.Println("Starting with empty Redis database")
	}

	return nil
}
