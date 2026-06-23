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
	"go-agent-chat-server/internal/metrics"
	"go-agent-chat-server/internal/model"
	"go-agent-chat-server/internal/pkg/idgen"
	"go-agent-chat-server/internal/queue"
	"go-agent-chat-server/internal/tool"
)

type ChatService struct {
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
		maxConcurrentLLM = 2
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

type ChatResult struct {
	SessionID          string `json:"session_id"`
	Model              string `json:"model"`
	Answer             string `json:"answer"`
	AssistantMessageID string `json:"assistant_message_id,omitempty"`
}

func (s *ChatService) acquireLLM(ctx context.Context) (func(), error) {
	start := time.Now()
	metrics.LLMAcquireTotal.Add(1)

	select {
	case s.llmLimiter <- struct{}{}:
		metrics.LLMAcquireWaitMsTotal.Add(metrics.DurationMs(time.Since(start)))
		metrics.LLMInFlight.Add(1)
		return func() {
			<-s.llmLimiter
			metrics.LLMInFlight.Add(-1)
		}, nil
	case <-ctx.Done():
		metrics.LLMAcquireRejectedTotal.Add(1)
		return nil, ctx.Err()
	}
}

func (s *ChatService) checkUserLLMRateLimit(ctx context.Context, userID string) error {
	metrics.UserLLMRateLimitCheckedTotal.Add(1)
	allowed, _, err := cache.AllowUserLLMCall(ctx, userID, int64(s.userLLMRateLimit), s.userLLMRateWindow)
	if err != nil {
		metrics.UserLLMRateLimitErrorTotal.Add(1)
		return err
	}
	if !allowed {
		metrics.UserLLMRateLimitBlockedTotal.Add(1)
		return ErrTooManyRequests
	}
	return nil
}

func (s *ChatService) Chat(ctx context.Context, userID, sessionID, modelName, userMessage string) (ChatResult, error) {
	return s.chatWithContext(ctx, userID, sessionID, modelName, userMessage, "")
}

func (s *ChatService) ChatWithExtraContext(ctx context.Context, userID, sessionID, modelName, userMessage, extraContext string) (ChatResult, error) {
	return s.chatWithContext(ctx, userID, sessionID, modelName, userMessage, extraContext)
}

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

	llmStart := time.Now()
	metrics.LLMCallTotal.Add(1)
	answer, err := s.chatModel(llmCtx, modelName, messages)
	metrics.LLMCallDurationMsTotal.Add(metrics.DurationMs(time.Since(llmStart)))
	if err != nil {
		metrics.LLMCallFailedTotal.Add(1)
		return ChatResult{}, err
	}

	assistantMsg, err := s.messageService.CreateMessage(ctx, userID, sessionID, "assistant", answer)
	if err != nil {
		return ChatResult{}, err
	}

	s.publishChatEvent("chat.completed", userID, sessionID, modelName, userMessage, answer, assistantMsg.ID)

	return ChatResult{
		SessionID:          sessionID,
		Model:              modelName,
		Answer:             answer,
		AssistantMessageID: assistantMsg.ID,
	}, nil
}

func (s *ChatService) GenerateAnswerFromHistory(ctx context.Context, userID, sessionID, modelName, userMessage string) (ChatResult, error) {
	if userID == "" {
		return ChatResult{}, errors.New("user_id is required")
	}
	if sessionID == "" {
		return ChatResult{}, errors.New("session_id is required")
	}
	if strings.TrimSpace(userMessage) == "" {
		return ChatResult{}, errors.New("message is required")
	}

	messages, err := s.messageService.ListMessages(ctx, userID, sessionID)
	if err != nil {
		return ChatResult{}, err
	}
	messages = s.injectSystemContext(ctx, userID, sessionID, userMessage, "", messages)

	llmCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	release, err := s.acquireLLM(llmCtx)
	if err != nil {
		return ChatResult{}, err
	}
	defer release()

	llmStart := time.Now()
	metrics.LLMCallTotal.Add(1)
	answer, err := s.chatModel(llmCtx, modelName, messages)
	metrics.LLMCallDurationMsTotal.Add(metrics.DurationMs(time.Since(llmStart)))
	if err != nil {
		metrics.LLMCallFailedTotal.Add(1)
		return ChatResult{}, err
	}

	assistantMsg, err := s.messageService.CreateMessage(ctx, userID, sessionID, "assistant", answer)
	if err != nil {
		return ChatResult{}, err
	}

	s.publishChatEvent("chat.async.completed", userID, sessionID, modelName, userMessage, answer, assistantMsg.ID)

	return ChatResult{
		SessionID:          sessionID,
		Model:              modelName,
		Answer:             answer,
		AssistantMessageID: assistantMsg.ID,
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

	llmStart := time.Now()
	metrics.LLMCallTotal.Add(1)
	answer, err := s.streamModel(llmCtx, modelName, messages, onDelta)
	metrics.LLMCallDurationMsTotal.Add(metrics.DurationMs(time.Since(llmStart)))
	if err != nil {
		metrics.LLMCallFailedTotal.Add(1)
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

	llmStart := time.Now()
	metrics.LLMCallTotal.Add(1)
	answer, err := s.chatCompletionModel(llmCtx, modelName, messages)
	metrics.LLMCallDurationMsTotal.Add(metrics.DurationMs(time.Since(llmStart)))
	if err != nil {
		metrics.LLMCallFailedTotal.Add(1)
		return "", err
	}
	return answer, nil
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

	llmStart := time.Now()
	metrics.LLMCallTotal.Add(1)
	answer, err := s.streamModel(llmCtx, modelName, messages, onDelta)
	metrics.LLMCallDurationMsTotal.Add(metrics.DurationMs(time.Since(llmStart)))
	if err != nil {
		metrics.LLMCallFailedTotal.Add(1)
		return answer, err
	}
	return answer, nil
}

func (s *ChatService) chatModel(ctx context.Context, modelName string, messages []model.Message) (string, error) {
	if isMockModel(modelName) {
		return mockChat(ctx, messages)
	}
	return s.einoRunner.Chat(ctx, modelName, messages)
}

func (s *ChatService) chatCompletionModel(ctx context.Context, modelName string, messages []model.Message) (string, error) {
	if isMockModel(modelName) {
		return mockChat(ctx, messages)
	}
	return s.ollamaClient.Chat(ctx, modelName, messages)
}

func (s *ChatService) streamModel(ctx context.Context, modelName string, messages []model.Message, onDelta func(delta string) error) (string, error) {
	if isMockModel(modelName) {
		return mockStreamChat(ctx, messages, onDelta)
	}
	return s.ollamaClient.StreamChat(ctx, modelName, messages, onDelta)
}

func isMockModel(modelName string) bool {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	return modelName == "mock" || modelName == "mock-llm"
}

func mockChat(ctx context.Context, messages []model.Message) (string, error) {
	select {
	case <-time.After(120 * time.Millisecond):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return "mock answer: " + lastUserMessage(messages), nil
}

func mockStreamChat(ctx context.Context, messages []model.Message, onDelta func(delta string) error) (string, error) {
	answer := "mock answer: " + lastUserMessage(messages)
	chunks := []string{"mock ", "answer: ", lastUserMessage(messages)}
	for _, chunk := range chunks {
		select {
		case <-time.After(80 * time.Millisecond):
		case <-ctx.Done():
			return strings.TrimSpace(answer), ctx.Err()
		}
		if err := onDelta(chunk); err != nil {
			return strings.TrimSpace(answer), err
		}
	}
	return answer, nil
}

func lastUserMessage(messages []model.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && strings.TrimSpace(messages[i].Content) != "" {
			return truncateForContext(messages[i].Content, 80)
		}
	}
	return "ok"
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
