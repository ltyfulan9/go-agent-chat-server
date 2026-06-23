package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var userLLMRateLimitScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if tonumber(current) == 1 then
    redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`)

func AllowUserLLMCall(ctx context.Context, userID string, maxRequests int64, window time.Duration) (bool, int64, error) {
	if maxRequests <= 0 {
		return true, 0, nil
	}
	if window <= 0 {
		window = time.Minute
	}

	key := fmt.Sprintf("rate_limit:llm:user:%s", userID)
	count, err := userLLMRateLimitScript.Run(ctx, RDB, []string{key}, window.Milliseconds()).Int64()
	if err != nil {
		return false, 0, err
	}

	return count <= maxRequests, count, nil
}
