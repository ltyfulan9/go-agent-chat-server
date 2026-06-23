package middleware

import (
	"context"
	"fmt"
	"time"

	"go-agent-chat-server/internal/api"
	"go-agent-chat-server/internal/ecode"
	"go-agent-chat-server/internal/metrics"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/redis/go-redis/v9"
)

var rateLimitScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if tonumber(current) == 1 then
    redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`)

func RateLimit(rdb *redis.Client, maxRequests int64, window time.Duration) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		ip := c.ClientIP()
		key := fmt.Sprintf("rate_limit:ip:%s", ip)

		metrics.IPRateLimitCheckedTotal.Add(1)

		count, err := rateLimitScript.Run(
			ctx,
			rdb,
			[]string{key},
			window.Milliseconds(),
		).Int64()

		if err != nil {
			metrics.IPRateLimitErrorTotal.Add(1)
			c.JSON(consts.StatusInternalServerError, api.Error(ecode.CodeInternal, "rate limit error"))
			return
		}

		if count > maxRequests {
			metrics.IPRateLimitBlockedTotal.Add(1)
			c.JSON(consts.StatusTooManyRequests, api.Error(ecode.CodeTooManyRequests, "too many requests"))
			return
		}

		c.Next(ctx)
	}
}
