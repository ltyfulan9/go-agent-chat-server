package api

import (
	"context"
	"strings"
	"time"

	"go-agent-chat-server/internal/ecode"
	"go-agent-chat-server/internal/model"
	"go-agent-chat-server/internal/pkg/idgen"
	"go-agent-chat-server/internal/queue"
	"go-agent-chat-server/internal/service"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type CozeCampusAskRequest struct {
	UserID         string                 `json:"user_id"`
	CozeUserID     string                 `json:"coze_user_id"`
	SessionID      string                 `json:"session_id"`
	Query          string                 `json:"query"`
	Intent         string                 `json:"intent"`
	HistorySummary string                 `json:"history_summary"`
	ExtraContext   string                 `json:"extra_context"`
	Model          string                 `json:"model"`
	TopK           int                    `json:"top_k"`
	Metadata       map[string]interface{} `json:"metadata"`
}

type CozeCampusSearchRequest struct {
	UserID     string `json:"user_id"`
	CozeUserID string `json:"coze_user_id"`
	Query      string `json:"query"`
	Intent     string `json:"intent"`
	TopK       int    `json:"top_k"`
}

type CozeEventRequest struct {
	Type       string                 `json:"type"`
	UserID     string                 `json:"user_id"`
	CozeUserID string                 `json:"coze_user_id"`
	SessionID  string                 `json:"session_id"`
	Payload    map[string]interface{} `json:"payload"`
}

type CozeSource struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

type CozeHandler struct {
	knowledgeService *service.KnowledgeService
	chatService      *service.ChatService
	eventPublisher   queue.Publisher
	defaultUserID    string
}

func NewCozeHandler(
	knowledgeService *service.KnowledgeService,
	chatService *service.ChatService,
	eventPublisher queue.Publisher,
	defaultUserID string,
) *CozeHandler {
	if eventPublisher == nil {
		eventPublisher = queue.NewNoopPublisher()
	}
	if strings.TrimSpace(defaultUserID) == "" {
		defaultUserID = "coze-default-user"
	}
	return &CozeHandler{
		knowledgeService: knowledgeService,
		chatService:      chatService,
		eventPublisher:   eventPublisher,
		defaultUserID:    defaultUserID,
	}
}

func (h *CozeHandler) CampusSearch(ctx context.Context, c *app.RequestContext) {
	var req CozeCampusSearchRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid request"))
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "query is required"))
		return
	}

	topK := normalizeTopK(req.TopK)
	userID := h.resolveUserID(req.UserID)
	docs, err := h.knowledgeService.SearchDocs(userID, query, topK)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, Error(ecode.CodeInternal, "search campus knowledge failed"))
		return
	}

	c.JSON(consts.StatusOK, Success(map[string]interface{}{
		"query":        query,
		"intent":       req.Intent,
		"coze_user_id": req.CozeUserID,
		"user_id":      userID,
		"need_web":     shouldCozeNeedWeb(query, req.Intent, docs),
		"sources":      buildCozeSources(docs),
		"matched_docs": docs,
	}))
}

func (h *CozeHandler) CampusAsk(ctx context.Context, c *app.RequestContext) {
	var req CozeCampusAskRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid request"))
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "query is required"))
		return
	}

	topK := normalizeTopK(req.TopK)
	userID := h.resolveUserID(req.UserID)
	docs, err := h.knowledgeService.SearchDocs(userID, query, topK)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, Error(ecode.CodeInternal, "search campus knowledge failed"))
		return
	}

	extraContext := buildCozeContext(req, docs)
	var answer string
	var sessionID string

	if strings.TrimSpace(req.SessionID) != "" {
		result, err := h.chatService.ChatWithExtraContext(ctx, userID, req.SessionID, req.Model, query, extraContext)
		if err != nil {
			writeChatError(c, err)
			return
		}
		answer = result.Answer
		sessionID = result.SessionID
	} else {
		messages := []model.Message{
			{
				Role:    "system",
				Content: buildCozeSystemPrompt(extraContext),
			},
			{
				Role:    "user",
				Content: query,
			},
		}
		answer, err = h.chatService.ChatCompletion(ctx, userID, req.Model, messages)
		if err != nil {
			writeChatError(c, err)
			return
		}
	}

	h.publishCozeEvent(ctx, "coze.campus.ask.completed", userID, sessionID, map[string]interface{}{
		"coze_user_id":  req.CozeUserID,
		"query":         query,
		"intent":        req.Intent,
		"top_k":         topK,
		"source_count":  len(docs),
		"answer_length": len([]rune(answer)),
		"metadata":      req.Metadata,
	})

	c.JSON(consts.StatusOK, Success(map[string]interface{}{
		"answer":       answer,
		"query":        query,
		"intent":       req.Intent,
		"coze_user_id": req.CozeUserID,
		"user_id":      userID,
		"session_id":   sessionID,
		"need_web":     shouldCozeNeedWeb(query, req.Intent, docs),
		"sources":      buildCozeSources(docs),
		"matched_docs": docs,
	}))
}

func (h *CozeHandler) Event(ctx context.Context, c *app.RequestContext) {
	var req CozeEventRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid request"))
		return
	}

	eventType := strings.TrimSpace(req.Type)
	if eventType == "" {
		eventType = "coze.event"
	}
	userID := h.resolveUserID(req.UserID)

	payload := req.Payload
	if payload == nil {
		payload = map[string]interface{}{}
	}
	payload["coze_user_id"] = req.CozeUserID

	event := queue.Event{
		ID:        idgen.NewID(),
		Type:      eventType,
		UserID:    userID,
		SessionID: req.SessionID,
		CreatedAt: time.Now(),
		Payload:   payload,
	}

	publishCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := h.eventPublisher.Publish(publishCtx, event); err != nil {
		c.JSON(consts.StatusInternalServerError, Error(ecode.CodeInternal, "publish coze event failed"))
		return
	}

	c.JSON(consts.StatusOK, Success(map[string]interface{}{
		"published": true,
		"event_id":  event.ID,
	}))
}

func (h *CozeHandler) resolveUserID(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID != "" {
		return userID
	}
	return h.defaultUserID
}

func (h *CozeHandler) publishCozeEvent(ctx context.Context, eventType string, userID string, sessionID string, payload map[string]interface{}) {
	if h.eventPublisher == nil {
		return
	}
	go func() {
		publishCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = h.eventPublisher.Publish(publishCtx, queue.Event{
			ID:        idgen.NewID(),
			Type:      eventType,
			UserID:    userID,
			SessionID: sessionID,
			CreatedAt: time.Now(),
			Payload:   payload,
		})
	}()
}

func normalizeTopK(topK int) int {
	if topK <= 0 {
		return 5
	}
	if topK > 10 {
		return 10
	}
	return topK
}

func buildCozeContext(req CozeCampusAskRequest, docs []model.KnowledgeDoc) string {
	parts := make([]string, 0)
	parts = append(parts, "You are 云小灵, a campus intelligent assistant for Yunnan University. Answer clearly, accurately, and avoid fabricating unsupported details.")

	if strings.TrimSpace(req.Intent) != "" {
		parts = append(parts, "Intent: "+strings.TrimSpace(req.Intent))
	}
	if strings.TrimSpace(req.HistorySummary) != "" {
		parts = append(parts, "Conversation history summary:\n"+strings.TrimSpace(req.HistorySummary))
	}
	if strings.TrimSpace(req.ExtraContext) != "" {
		parts = append(parts, "Extra context from Coze workflow:\n"+strings.TrimSpace(req.ExtraContext))
	}
	parts = append(parts, "Campus knowledge documents:\n"+buildKnowledgeContext(docs))
	return strings.Join(parts, "\n\n")
}

func buildCozeSystemPrompt(extraContext string) string {
	return "Use the following campus context when it is relevant. If the context is insufficient or the question asks for latest deadlines/notices, say that official sources or web search should be checked.\n\n" + extraContext
}

func buildCozeSources(docs []model.KnowledgeDoc) []CozeSource {
	sources := make([]CozeSource, 0, len(docs))
	for _, doc := range docs {
		sources = append(sources, CozeSource{
			ID:      doc.ID,
			Title:   doc.Title,
			Snippet: truncateCozeSnippet(doc.Content, 180),
		})
	}
	return sources
}

func truncateCozeSnippet(value string, max int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "..."
}

func shouldCozeNeedWeb(query string, intent string, docs []model.KnowledgeDoc) bool {
	text := strings.ToLower(query + " " + intent)
	if len(docs) == 0 {
		return true
	}
	keywords := []string{"最新", "今年", "现在", "截止", "报名时间", "通知", "官网", "latest", "deadline", "2026", "2025"}
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}
