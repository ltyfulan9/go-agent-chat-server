package api

import (
	"context"
	"fmt"
	"strings"

	"go-agent-chat-server/internal/ecode"
	"go-agent-chat-server/internal/model"
	"go-agent-chat-server/internal/service"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type CreateKnowledgeDocRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type ChatWithKnowledgeRequest struct {
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
	Message   string `json:"message"`
	TopK      int    `json:"top_k"`
}

type KnowledgeHandler struct {
	knowledgeService *service.KnowledgeService
	chatService      *service.ChatService
}

func NewKnowledgeHandler(knowledgeService *service.KnowledgeService, chatService *service.ChatService) *KnowledgeHandler {
	return &KnowledgeHandler{
		knowledgeService: knowledgeService,
		chatService:      chatService,
	}
}

func (h *KnowledgeHandler) CreateDoc(ctx context.Context, c *app.RequestContext) {
	var req CreateKnowledgeDocRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid request"))
		return
	}

	doc, err := h.knowledgeService.CreateDoc(CurrentUserID(c), req.Title, req.Content)
	if err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidParam, err.Error()))
		return
	}

	c.JSON(consts.StatusOK, Success(doc))
}

func (h *KnowledgeHandler) ListDocs(ctx context.Context, c *app.RequestContext) {
	page, pageSize := ParsePageQuery(c)
	docs, total, err := h.knowledgeService.ListDocs(CurrentUserID(c), page, pageSize)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, Error(ecode.CodeInternal, "list knowledge docs failed"))
		return
	}

	c.JSON(consts.StatusOK, Success(PageData{
		Items:    docs,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}))
}

func (h *KnowledgeHandler) GetDoc(ctx context.Context, c *app.RequestContext) {
	doc, err := h.knowledgeService.GetDoc(CurrentUserID(c), c.Param("id"))
	if err != nil {
		c.JSON(consts.StatusNotFound, Error(ecode.CodeNotFound, "knowledge doc not found"))
		return
	}
	c.JSON(consts.StatusOK, Success(doc))
}

func (h *KnowledgeHandler) DeleteDoc(ctx context.Context, c *app.RequestContext) {
	if err := h.knowledgeService.DeleteDoc(CurrentUserID(c), c.Param("id")); err != nil {
		c.JSON(consts.StatusNotFound, Error(ecode.CodeNotFound, "knowledge doc not found"))
		return
	}
	c.JSON(consts.StatusOK, Success(map[string]bool{"deleted": true}))
}

func (h *KnowledgeHandler) ChatWithKnowledge(ctx context.Context, c *app.RequestContext) {
	var req ChatWithKnowledgeRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid request"))
		return
	}
	if req.SessionID == "" || strings.TrimSpace(req.Message) == "" {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "session_id and message are required"))
		return
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}
	if req.TopK > 10 {
		req.TopK = 10
	}

	userID := CurrentUserID(c)
	docs, err := h.knowledgeService.SearchDocs(userID, req.Message, req.TopK)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, Error(ecode.CodeInternal, "search knowledge docs failed"))
		return
	}

	extraContext := buildKnowledgeContext(docs)
	result, err := h.chatService.ChatWithExtraContext(ctx, userID, req.SessionID, req.Model, req.Message, extraContext)
	if err != nil {
		writeChatError(c, err)
		return
	}

	c.JSON(consts.StatusOK, Success(map[string]interface{}{
		"chat":         result,
		"matched_docs": docs,
	}))
}

func buildKnowledgeContext(docs []model.KnowledgeDoc) string {
	if len(docs) == 0 {
		return "No matching knowledge documents were found."
	}

	parts := make([]string, 0, len(docs)) //创建切片
	for _, doc := range docs {
		content := doc.Content
		runes := []rune(content)
		if len(runes) > 1000 {
			content = string(runes[:1000]) + "..."
		}
		parts = append(parts, fmt.Sprintf("[Doc: %s]\n%s", doc.Title, content))
	}
	return strings.Join(parts, "\n\n")
}
