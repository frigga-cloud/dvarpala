package redis

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/yourcompany/dvarpala/internal/config"
)

type Client struct {
	*redis.Client
}

func NewClient(cfg config.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// Test connection
	_, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &Client{Client: rdb}, nil
}