package api

import (
	"context"
	"errors"

	"go-agent-chat-server/internal/ecode"
	"go-agent-chat-server/internal/service"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type AsyncChatHandler struct {
	asyncChatService *service.AsyncChatService
}

func NewAsyncChatHandler(asyncChatService *service.AsyncChatService) *AsyncChatHandler {
	return &AsyncChatHandler{asyncChatService: asyncChatService}
}

func (h *AsyncChatHandler) Submit(ctx context.Context, c *app.RequestContext) {
	var req ChatRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid request"))
		return
	}

	job, err := h.asyncChatService.Submit(ctx, CurrentUserID(c), req.SessionID, req.Model, req.Message)
	if err != nil {
		writeAsyncChatError(c, err)
		return
	}

	c.JSON(202, Success(map[string]interface{}{
		"job_id":     job.ID,
		"status":     job.Status,
		"session_id": job.SessionID,
		"model":      job.Model,
	}))
}

func (h *AsyncChatHandler) GetJob(ctx context.Context, c *app.RequestContext) {
	job, err := h.asyncChatService.GetJob(ctx, CurrentUserID(c), c.Param("id"))
	if err != nil {
		c.JSON(consts.StatusNotFound, Error(ecode.CodeNotFound, "chat job not found"))
		return
	}

	c.JSON(consts.StatusOK, Success(job))
}

func writeAsyncChatError(c *app.RequestContext, err error) {
	if errors.Is(err, service.ErrAsyncQueueDisabled) {
		c.JSON(503, Error(ecode.CodeInternal, "async chat queue is disabled"))
		return
	}
	if errors.Is(err, service.ErrTooManyRequests) {
		c.JSON(consts.StatusTooManyRequests, Error(ecode.CodeTooManyRequests, "too many llm requests"))
		return
	}
	c.JSON(consts.StatusInternalServerError, Error(ecode.CodeInternal, err.Error()))
}
