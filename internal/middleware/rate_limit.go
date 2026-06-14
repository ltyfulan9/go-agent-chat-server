package middleware

import (
	"context"
	"fmt"
	"time"

	"go-agent-chat-server/internal/api"
	"go-agent-chat-server/internal/ecode"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/redis/go-redis/v9"
)

//限制同一个IP在一段时间内只能发出一定数量的请求，防止恶意攻击或者过于频繁的请求导致服务器压力过大，超过返回429 Too Many Requests错误

var rateLimitScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if tonumber(current) == 1 then
    redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`)

//Lua脚本，原子性地执行INCR和PEXPIRE命令，确保在高并发情况下也能正确统计请求次数和设置过期时间

func RateLimit(rdb *redis.Client, maxRequests int64, window time.Duration) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		ip := c.ClientIP() //获取当前请求的客户端IP、然后在Redis里用这个IP作为key来统计请求次数
		key := fmt.Sprintf("rate_limit:ip:%s", ip)

		count, err := rateLimitScript.Run(
			ctx,
			rdb,
			[]string{key},
			window.Milliseconds(),
		).Int64()

		if err != nil {
			c.JSON(consts.StatusInternalServerError, api.Error(ecode.CodeInternal, "rate limit error"))
			return
		}
		//Redis出错久返回500
		if count > maxRequests {
			c.JSON(consts.StatusTooManyRequests, api.Error(ecode.CodeTooManyRequests, "too many requests"))
			return
		}

		c.Next(ctx)
	}
}

//同一个IP，1分钟内最多请求60次
