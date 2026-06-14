package middleware

import (
	"context"
	"crypto/subtle"
	"strings"

	"go-agent-chat-server/internal/ecode"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// CozeAPIKey protects APIs that are called by Coze HTTP nodes or custom plugins.
// If apiKey is empty, the middleware is disabled for local development.
func CozeAPIKey(apiKey string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if strings.TrimSpace(apiKey) == "" {
			c.Next(ctx)
			return
		}

		provided := strings.TrimSpace(string(c.GetHeader("X-Coze-API-Key")))
		if provided == "" {
			provided = strings.TrimSpace(string(c.GetHeader("X-API-Key")))
		}
		if provided == "" {
			authorization := strings.TrimSpace(string(c.GetHeader("Authorization")))
			if strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
				provided = strings.TrimSpace(authorization[len("Bearer "):])
			}
		}

		if subtle.ConstantTimeCompare([]byte(provided), []byte(apiKey)) != 1 {
			c.JSON(consts.StatusUnauthorized, map[string]interface{}{
				"code":    ecode.CodeUnauthorized,
				"message": "invalid coze api key",
			})
			return
		}

		c.Next(ctx)
	}
}
