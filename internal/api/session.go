package api

import (
	"context"
	"strings"

	"go-agent-chat-server/internal/ecode"
	"go-agent-chat-server/internal/service"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type CreateSessionRequest struct {
	Title string `json:"title"`
}

type UpdateSessionRequest struct {
	Title string `json:"title"`
}

type SessionHandler struct {
	sessionService *service.SessionService
}

func NewSessionHandler(sessionService *service.SessionService) *SessionHandler {
	return &SessionHandler{
		sessionService: sessionService,
	}
}

func (h *SessionHandler) CreateSession(ctx context.Context, c *app.RequestContext) {
	var req CreateSessionRequest

	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid request"))
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		req.Title = "New Chat"
	}

	userID := CurrentUserID(c)
	session, err := h.sessionService.CreateSession(userID, req.Title)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, Error(ecode.CodeInternal, "create session failed"))
		return
	}

	c.JSON(consts.StatusOK, Success(session))
}

func (h *SessionHandler) GetSession(ctx context.Context, c *app.RequestContext) {
	id := c.Param("id")
	userID := CurrentUserID(c)

	session, err := h.sessionService.GetSession(userID, id)
	if err != nil {
		c.JSON(consts.StatusNotFound, Error(ecode.CodeNotFound, "session not found"))
		return
	}

	c.JSON(consts.StatusOK, Success(session))
}

func (h *SessionHandler) ListSessions(ctx context.Context, c *app.RequestContext) {
	userID := CurrentUserID(c)

	page, pageSize := ParsePageQuery(c)

	sessions, total, err := h.sessionService.ListSessions(userID, page, pageSize)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, Error(ecode.CodeInternal, "list sessions failed"))
		return
	}

	c.JSON(consts.StatusOK, Success(PageData{
		Items:    sessions,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}))
}

func (h *SessionHandler) UpdateSession(ctx context.Context, c *app.RequestContext) {
	id := c.Param("id")
	userID := CurrentUserID(c)

	var req UpdateSessionRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid request"))
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidParam, "title is required"))
		return
	}

	session, err := h.sessionService.UpdateSessionTitle(userID, id, req.Title)
	if err != nil {
		c.JSON(consts.StatusNotFound, Error(ecode.CodeNotFound, "session not found"))
		return
	}

	c.JSON(consts.StatusOK, Success(session))
}

func (h *SessionHandler) DeleteSession(ctx context.Context, c *app.RequestContext) {
	id := c.Param("id")
	userID := CurrentUserID(c)

	if err := h.sessionService.DeleteSession(ctx, userID, id); err != nil {
		c.JSON(consts.StatusNotFound, Error(ecode.CodeNotFound, "session not found"))
		return
	}

	c.JSON(consts.StatusOK, Success(map[string]bool{"deleted": true}))
}
