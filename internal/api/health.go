package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func Health(ctx context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusOK, Success(map[string]string{
		"status": "healthy",
	}))
}
