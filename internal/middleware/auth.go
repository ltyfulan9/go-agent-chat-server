package middleware

import (
	"context"
	"strings"

	"go-agent-chat-server/internal/api"
	"go-agent-chat-server/internal/ecode"
	"go-agent-chat-server/internal/pkg/jwtutil"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func Auth(jwtSecret string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		authHeader := string(c.Request.Header.Peek("Authorization"))
		if authHeader == "" {
			c.JSON(consts.StatusUnauthorized, api.Error(ecode.CodeMissingAuthorization, "missing authorization header"))
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			c.JSON(consts.StatusUnauthorized, api.Error(ecode.CodeUnauthorized, "invalid authorization header"))
			return
		}

		claims, err := jwtutil.ParseToken(parts[1], jwtSecret)
		if err != nil {
			c.JSON(consts.StatusUnauthorized, api.Error(ecode.CodeInvalidToken, "invalid token: "+err.Error()))
			return
		}

		c.Set(api.CurrentUserIDKey, claims.UserID)
		c.Next(ctx)
	}
}
