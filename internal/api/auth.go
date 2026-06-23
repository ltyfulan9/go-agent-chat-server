package api

import (
	"context"

	"go-agent-chat-server/internal/ecode"
	"go-agent-chat-server/internal/service"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(ctx context.Context, c *app.RequestContext) {
	var req AuthRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid request"))
		return
	}

	result, err := h.authService.Register(req.Username, req.Password)
	if err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidParam, err.Error()))
		return
	}

	c.JSON(consts.StatusOK, Success(result))
}

func (h *AuthHandler) Login(ctx context.Context, c *app.RequestContext) {
	var req AuthRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid request"))
		return
	}

	result, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(consts.StatusUnauthorized, Error(ecode.CodeUnauthorized, err.Error()))
		return
	}

	c.JSON(consts.StatusOK, Success(result))
}
