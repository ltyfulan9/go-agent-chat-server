package api

import (
	"context"
	"time"

	"go-agent-chat-server/internal/cache"
	"go-agent-chat-server/internal/ecode"
	"go-agent-chat-server/internal/store"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func Health(ctx context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusOK, Success(map[string]string{
		"status": "healthy",
	}))
}

func Ready(ctx context.Context, c *app.RequestContext) {
	readyCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	checks := map[string]string{
		"mysql": "ok",
		"redis": "ok",
	}

	if store.DB == nil {
		checks["mysql"] = "not_initialized"
	} else if sqlDB, err := store.DB.DB(); err != nil {
		checks["mysql"] = err.Error()
	} else if err := sqlDB.PingContext(readyCtx); err != nil {
		checks["mysql"] = err.Error()
	}

	if cache.RDB == nil {
		checks["redis"] = "not_initialized"
	} else if err := cache.RDB.Ping(readyCtx).Err(); err != nil {
		checks["redis"] = err.Error()
	}

	if checks["mysql"] != "ok" || checks["redis"] != "ok" {
		c.JSON(consts.StatusServiceUnavailable, Response{Code: ecode.CodeInternal, Message: "service not ready", Data: checks})
		return
	}

	c.JSON(consts.StatusOK, Success(map[string]interface{}{
		"status": "ready",
		"checks": checks,
	}))
}
