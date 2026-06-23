package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-agent-chat-server/internal/model"

	"github.com/redis/go-redis/v9"
)

const messageCacheTTL = 5 * time.Minute

func messageCacheKey(sessionID string) string {
	return fmt.Sprintf("session:messages:%s", sessionID)
}

func GetMessages(ctx context.Context, sessionID string) ([]model.Message, bool, error) {
	key := messageCacheKey(sessionID)

	value, err := RDB.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var messages []model.Message
	if err := json.Unmarshal([]byte(value), &messages); err != nil {
		return nil, false, err
	}

	return messages, true, nil
}

func SetMessages(ctx context.Context, sessionID string, messages []model.Message) error {
	key := messageCacheKey(sessionID)

	if messages == nil {
		messages = []model.Message{}
	}

	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}

	return RDB.Set(ctx, key, data, messageCacheTTL).Err()
}

func DeleteMessages(ctx context.Context, sessionID string) error {
	key := messageCacheKey(sessionID)
	return RDB.Del(ctx, key).Err()
}
