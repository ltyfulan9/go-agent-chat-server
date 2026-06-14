package middleware

import (
	"context"
	"log"
	"time"

	"go-agent-chat-server/internal/api"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func Logger() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		method := string(c.Request.Header.Method())
		path := string(c.Request.URI().Path())
		clientIP := c.ClientIP()

		c.Next(ctx)

		status := c.Response.StatusCode()
		if status == 0 {
			status = consts.StatusOK
		}

		requestID := ""
		if value, ok := c.Get(RequestIDKey); ok {
			if s, ok := value.(string); ok {
				requestID = s
			}
		}

		userID := ""
		if value, ok := c.Get(api.CurrentUserIDKey); ok {
			if s, ok := value.(string); ok {
				userID = s
			}
		}

		log.Printf(
			"request_id=%s method=%s path=%s status=%d latency=%s client_ip=%s user_id=%s",
			requestID,
			method,
			path,
			status,
			time.Since(start),
			clientIP,
			userID,
		)
	}
}
