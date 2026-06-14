package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"go-agent-chat-server/internal/agent"
	"go-agent-chat-server/internal/cache"
	"go-agent-chat-server/internal/llm"
	"go-agent-chat-server/internal/model"
	"go-agent-chat-server/internal/pkg/idgen"
	"go-agent-chat-server/internal/queue"
	"go-agent-chat-server/internal/tool"
)

type ChatService struct { //夹在API Handler和底层能力中间
	messageService    *MessageService
	einoRunner        *agent.EinoRunner
	ollamaClient      *llm.OllamaClient
	llmLimiter        chan struct{}
	userLLMRateLimit  int
	userLLMRateWindow time.Duration
	toolRegistry      *tool.Registry
	eventPublisher    queue.Publisher
}

func NewChatService(
	messageService *MessageService,
	einoRunner *agent.EinoRunner,
	ollamaClient *llm.OllamaClient,
	maxConcurrentLLM int,
	userLLMRateLimit int,
	userLLMRateWindow time.Duration,
	eventPublisher queue.Publisher,
) *ChatService {
	if maxConcurrentLLM <= 0 {
		maxConcurrentLLM = 10
	}
	if userLLMRateWindow <= 0 {
		userLLMRateWindow = time.Minute
	}
	if eventPublisher == nil {
		eventPublisher = queue.NewNoopPublisher()
	}

	return &ChatService{
		messageService:    messageService,
		einoRunner:        einoRunner,
		ollamaClient:      ollamaClient,
		llmLimiter:        make(chan struct{}, maxConcurrentLLM),
		userLLMRateLimit:  userLLMRateLimit,
		userLLMRateWindow: userLLMRateWindow,
		toolRegistry:      tool.NewDefaultRegistry(),
		eventPublisher:    eventPublisher,
	}
}

type ChatResult struct { //返回给前端的数据结构
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
	Answer    string `json:"answer"`
}

func (s *ChatService) acquireLLM(ctx context.Context) (func(), error) { //申请一个LLM调用的权限，返回一个释放函数
	select {
	case s.llmLimiter <- struct{}{}:
		return func() {
			<-s.llmLimiter
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *ChatService) checkUserLLMRateLimit(ctx context.Context, userID string) error {
	allowed, _, err := cache.AllowUserLLMCall(ctx, userID, int64(s.userLLMRateLimit), s.userLLMRateWindow)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrTooManyRequests
	}
	return nil
}

func (s *ChatService) Chat(ctx context.Context, userID, sessionID, modelName, userMessage string) (ChatResult, error) {
	return s.chatWithContext(ctx, userID, sessionID, modelName, userMessage, "")
}

// 没有额外知识上下文
func (s *ChatService) ChatWithExtraContext(ctx context.Context, userID, sessionID, modelName, userMessage, extraContext string) (ChatResult, error) {
	return s.chatWithContext(ctx, userID, sessionID, modelName, userMessage, extraContext)
}

// 可以额外传入比如知识库检索结果
func (s *ChatService) chatWithContext(ctx context.Context, userID, sessionID, modelName, userMessage, extraContext string) (ChatResult, error) {
	if userID == "" {
		return ChatResult{}, errors.New("user_id is required")
	}

	if sessionID == "" {
		return ChatResult{}, errors.New("session_id is required")
	}

	if strings.TrimSpace(userMessage) == "" {
		return ChatResult{}, errors.New("message is required")
	}

	if err := s.checkUserLLMRateLimit(ctx, userID); err != nil {
		return ChatResult{}, err
	}

	if _, err := s.messageService.CreateMessage(ctx, userID, sessionID, "user", userMessage); err != nil {
		return ChatResult{}, err
	}

	messages, err := s.messageService.ListMessages(ctx, userID, sessionID)
	if err != nil {
		return ChatResult{}, err
	}

	messages = s.injectSystemContext(ctx, userID, sessionID, userMessage, extraContext, messages)

	llmCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	release, err := s.acquireLLM(llmCtx)
	if err != nil {
		return ChatResult{}, err
	}
	defer release()

	answer, err := s.einoRunner.Chat(llmCtx, modelName, messages)
	if err != nil {
		return ChatResult{}, err
	}

	assistantMsg, err := s.messageService.CreateMessage(ctx, userID, sessionID, "assistant", answer)
	if err != nil {
		return ChatResult{}, err
	}

	s.publishChatEvent("chat.completed", userID, sessionID, modelName, userMessage, answer, assistantMsg.ID)

	return ChatResult{
		SessionID: sessionID,
		Model:     modelName,
		Answer:    answer,
	}, nil
}

func (s *ChatService) StreamChat(
	ctx context.Context,
	userID string,
	sessionID string,
	modelName string,
	userMessage string,
	onDelta func(delta string) error,
) (string, error) {
	if userID == "" {
		return "", errors.New("user_id is required")
	}

	if sessionID == "" {
		return "", errors.New("session_id is required")
	}

	if strings.TrimSpace(userMessage) == "" {
		return "", errors.New("message is required")
	}

	if err := s.checkUserLLMRateLimit(ctx, userID); err != nil {
		return "", err
	}

	if _, err := s.messageService.CreateMessage(ctx, userID, sessionID, "user", userMessage); err != nil {
		return "", err
	}

	messages, err := s.messageService.ListMessages(ctx, userID, sessionID)
	if err != nil {
		return "", err
	}

	messages = s.injectSystemContext(ctx, userID, sessionID, userMessage, "", messages)

	llmCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	release, err := s.acquireLLM(llmCtx)
	if err != nil {
		return "", err
	}
	defer release()

	answer, err := s.ollamaClient.StreamChat(llmCtx, modelName, messages, onDelta)
	if err != nil {
		return answer, err
	}

	assistantMsg, err := s.messageService.CreateMessage(ctx, userID, sessionID, "assistant", answer)
	if err != nil {
		return answer, err
	}

	s.publishChatEvent("chat.stream.completed", userID, sessionID, modelName, userMessage, answer, assistantMsg.ID)

	return answer, nil
}

func (s *ChatService) ChatCompletion(ctx context.Context, userID string, modelName string, messages []model.Message) (string, error) {
	if userID == "" {
		return "", errors.New("user_id is required")
	}
	if len(messages) == 0 {
		return "", errors.New("messages are required")
	}
	if err := s.checkUserLLMRateLimit(ctx, userID); err != nil {
		return "", err
	}

	llmCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	release, err := s.acquireLLM(llmCtx)
	if err != nil {
		return "", err
	}
	defer release()

	return s.ollamaClient.Chat(llmCtx, modelName, messages)
}

func (s *ChatService) StreamChatCompletion(ctx context.Context, userID string, modelName string, messages []model.Message, onDelta func(delta string) error) (string, error) {
	if userID == "" {
		return "", errors.New("user_id is required")
	}
	if len(messages) == 0 {
		return "", errors.New("messages are required")
	}
	if err := s.checkUserLLMRateLimit(ctx, userID); err != nil {
		return "", err
	}

	llmCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	release, err := s.acquireLLM(llmCtx)
	if err != nil {
		return "", err
	}
	defer release()

	return s.ollamaClient.StreamChat(llmCtx, modelName, messages, onDelta)
}

func (s *ChatService) publishChatEvent(eventType, userID, sessionID, modelName, userMessage, answer, assistantMessageID string) {
	if s.eventPublisher == nil {
		return
	}

	event := queue.Event{
		ID:        idgen.NewID(),
		Type:      eventType,
		UserID:    userID,
		SessionID: sessionID,
		CreatedAt: time.Now(),
		Payload: map[string]interface{}{
			"model":                modelName,
			"user_message_length":  len([]rune(userMessage)),
			"answer_length":        len([]rune(answer)),
			"assistant_message_id": assistantMessageID,
		},
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := s.eventPublisher.Publish(ctx, event); err != nil {
			log.Printf("publish chat event failed: %v", err)
		}
	}()
}

func (s *ChatService) injectSystemContext(ctx context.Context, userID, sessionID, userMessage, extraContext string, messages []model.Message) []model.Message {
	parts := make([]string, 0)

	if strings.TrimSpace(extraContext) != "" {
		parts = append(parts, "External knowledge context:\n"+strings.TrimSpace(extraContext))
	}

	toolContext := s.buildToolContext(ctx, userID, sessionID, userMessage)
	if toolContext != "" {
		parts = append(parts, "Tool results:\n"+toolContext)
	}

	if len(parts) == 0 {
		return messages
	}

	systemMsg := model.Message{
		Role:    "system",
		Content: "Use the following context when it is relevant. Do not fabricate details not supported by the context.\n\n" + strings.Join(parts, "\n\n"),
	}

	combined := make([]model.Message, 0, len(messages)+1)
	combined = append(combined, systemMsg)
	combined = append(combined, messages...)
	return combined
}

func (s *ChatService) buildToolContext(ctx context.Context, userID, sessionID, userMessage string) string {
	results := make([]string, 0)

	for _, result := range s.toolRegistry.RunMatched(ctx, userMessage) {
		results = append(results, fmt.Sprintf("- %s: %s", result.Name, result.Output))
	}

	if shouldSearchSession(userMessage) {
		keyword := extractSearchKeyword(userMessage)
		messages, err := s.messageService.SearchMessages(ctx, userID, sessionID, keyword, 5)
		if err == nil && len(messages) > 0 {
			lines := make([]string, 0, len(messages))
			for _, m := range messages {
				lines = append(lines, fmt.Sprintf("%s: %s", m.Role, truncateForContext(m.Content, 200)))
			}
			results = append(results, "- session_search: "+strings.Join(lines, " | "))
		}
	}

	return strings.Join(results, "\n")
}

func shouldSearchSession(input string) bool {
	text := strings.ToLower(input)
	keywords := []string{"历史", "之前", "刚才", "上面", "前面", "history", "previous", "earlier", "search session"}
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func extractSearchKeyword(input string) string {
	keyword := strings.TrimSpace(input)
	replacers := []string{"历史", "之前", "刚才", "上面", "前面", "帮我", "查一下", "搜索", "查询", "history", "previous", "earlier", "search session"}
	for _, old := range replacers {
		keyword = strings.ReplaceAll(keyword, old, "")
	}
	keyword = strings.TrimSpace(keyword)
	if len([]rune(keyword)) > 32 {
		runes := []rune(keyword)
		keyword = string(runes[:32])
	}
	return keyword
}

func truncateForContext(value string, max int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "..."
}
