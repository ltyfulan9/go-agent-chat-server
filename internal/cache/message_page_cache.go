package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-agent-chat-server/internal/model"

	"github.com/redis/go-redis/v9"
)

const messagePageCacheTTL = 2 * time.Minute

type MessagePageCache struct {
	Items []model.Message `json:"items"`
	Total int64           `json:"total"`
}

func messagePageCacheKey(sessionID string, page int, pageSize int) string {
	return fmt.Sprintf("session:messages_page:%s:p:%d:s:%d", sessionID, page, pageSize)
}

func GetMessagePage(ctx context.Context, sessionID string, page int, pageSize int) ([]model.Message, int64, bool, error) {
	key := messagePageCacheKey(sessionID, page, pageSize)
	value, err := RDB.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}

	var cached MessagePageCache
	if err := json.Unmarshal([]byte(value), &cached); err != nil {
		return nil, 0, false, err
	}
	if cached.Items == nil {
		cached.Items = []model.Message{}
	}
	return cached.Items, cached.Total, true, nil
}

func SetMessagePage(ctx context.Context, sessionID string, page int, pageSize int, messages []model.Message, total int64) error {
	key := messagePageCacheKey(sessionID, page, pageSize)
	if messages == nil {
		messages = []model.Message{}
	}
	data, err := json.Marshal(MessagePageCache{Items: messages, Total: total})
	if err != nil {
		return err
	}
	return RDB.Set(ctx, key, data, messagePageCacheTTL).Err()
}

func DeleteMessagePageCaches(ctx context.Context, sessionID string) error {
	pattern := fmt.Sprintf("session:messages_page:%s:*", sessionID)
	var cursor uint64
	for {
		keys, next, err := RDB.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := RDB.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		if next == 0 {
			return nil
		}
		cursor = next
	}
}
