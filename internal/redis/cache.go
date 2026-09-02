package redis

import (
	"context"
	"time"
)

func (c *Client) SetWithExpiry(key string, value interface{}, expiration time.Duration) error {
	return c.Set(context.Background(), key, value, expiration).Err()
}

func (c *Client) GetValue(key string) (string, error) {
	return c.Get(context.Background(), key).Result()
}
