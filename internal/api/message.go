package api

import (
	"context"

	"go-agent-chat-server/internal/ecode"
	"go-agent-chat-server/internal/service"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type CreateMessageRequest struct {
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
}

type MessageHandler struct {
	messageService *service.MessageService
}

func NewMessageHandler(messageService *service.MessageService) *MessageHandler {
	return &MessageHandler{
		messageService: messageService,
	}
}

func (h *MessageHandler) CreateMessage(ctx context.Context, c *app.RequestContext) {
	var req CreateMessageRequest

	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid request"))
		return
	}

	userID := CurrentUserID(c)
	message, err := h.messageService.CreateMessage(ctx, userID, req.SessionID, req.Role, req.Content)
	if err != nil {
		if err.Error() == "session not found" {
			c.JSON(consts.StatusNotFound, Error(ecode.CodeNotFound, "session not found"))
			return
		}

		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidParam, err.Error()))
		return
	}

	c.JSON(consts.StatusOK, Success(message))
}

func (h *MessageHandler) ListMessages(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Param("id")
	userID := CurrentUserID(c)

	page, pageSize := ParsePageQuery(c)
	if pageSize == DefaultPageSize {
		pageSize = 30
	}

	messages, total, err := h.messageService.ListMessagesPage(ctx, userID, sessionID, page, pageSize)
	if err != nil {
		if err.Error() == "session not found" {
			c.JSON(consts.StatusNotFound, Error(ecode.CodeNotFound, "session not found"))
			return
		}

		c.JSON(consts.StatusInternalServerError, Error(ecode.CodeInternal, "list messages failed"))
		return
	}

	c.JSON(consts.StatusOK, Success(PageData{
		Items:    messages,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}))
}
