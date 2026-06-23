package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

func InitRedis(addr string, password string, db int) error {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("connect redis failed: %w", err)
	}

	RDB = rdb
	return nil
}
