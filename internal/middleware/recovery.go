package middleware

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"

	"go-agent-chat-server/internal/api"
	"go-agent-chat-server/internal/ecode"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func Recovery() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if err := recover(); err != nil {
				requestID := ""
				if value, ok := c.Get(RequestIDKey); ok {
					if s, ok := value.(string); ok {
						requestID = s
					}
				}

				log.Printf("panic recovered: request_id=%s error=%v stack=%s", requestID, err, debug.Stack())
				c.JSON(consts.StatusInternalServerError, api.Error(ecode.CodeInternal, fmt.Sprintf("internal server error, request_id=%s", requestID)))
			}
		}()

		c.Next(ctx)
	}
}
