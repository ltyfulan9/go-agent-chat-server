package router

import (
	"time"

	"go-agent-chat-server/internal/agent"
	"go-agent-chat-server/internal/api"
	"go-agent-chat-server/internal/cache"
	"go-agent-chat-server/internal/config"
	"go-agent-chat-server/internal/llm"
	"go-agent-chat-server/internal/middleware"
	"go-agent-chat-server/internal/queue"
	"go-agent-chat-server/internal/service"
	"go-agent-chat-server/internal/store"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func Register(h *server.Hertz, cfg *config.Config, eventPublisher queue.Publisher, taskPublisher queue.ChatTaskPublisher) {
	h.Use(middleware.Recovery())
	h.Use(middleware.RequestID())
	h.Use(middleware.Logger())
	h.Use(middleware.RateLimit(cache.RDB, int64(cfg.IPRateLimit), time.Duration(cfg.IPRateWindowSeconds)*time.Second))

	h.GET("/api/health", api.Health)
	h.GET("/api/ready", api.Ready)

	userRepo := store.NewUserRepo(store.DB)
	sessionRepo := store.NewSessionRepo(store.DB)
	messageRepo := store.NewMessageRepo(store.DB)
	knowledgeRepo := store.NewKnowledgeRepo(store.DB)
	chatJobRepo := store.NewChatJobRepo(store.DB)

	authService := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTExpireHours)
	sessionService := service.NewSessionService(sessionRepo)
	messageService := service.NewMessageService(messageRepo, sessionRepo)
	knowledgeService := service.NewKnowledgeService(knowledgeRepo)

	einoRunner := agent.NewEinoRunner(cfg.OllamaBaseURL, cfg.DefaultModel)
	ollamaClient := llm.NewOllamaClient(cfg.OllamaBaseURL, cfg.DefaultModel)

	chatService := service.NewChatService(
		messageService,
		einoRunner,
		ollamaClient,
		cfg.MaxConcurrentLLM,
		cfg.UserLLMRateLimit,
		time.Duration(cfg.UserLLMRateWindowSeconds)*time.Second,
		eventPublisher,
	)
	asyncChatService := service.NewAsyncChatService(chatJobRepo, chatService, taskPublisher, cfg.ChatJobMaxRetry)

	authHandler := api.NewAuthHandler(authService)
	sessionHandler := api.NewSessionHandler(sessionService)
	messageHandler := api.NewMessageHandler(messageService)
	chatHandler := api.NewChatHandler(chatService)
	asyncChatHandler := api.NewAsyncChatHandler(asyncChatService)
	knowledgeHandler := api.NewKnowledgeHandler(knowledgeService, chatService)
	cozeHandler := api.NewCozeHandler(knowledgeService, chatService, eventPublisher, cfg.CozeDefaultUserID)

	h.POST("/api/auth/register", authHandler.Register)
	h.POST("/api/auth/login", authHandler.Login)

	authMiddleware := middleware.Auth(cfg.JWTSecret)

	h.POST("/api/sessions", authMiddleware, sessionHandler.CreateSession)
	h.GET("/api/sessions", authMiddleware, sessionHandler.ListSessions)
	h.GET("/api/sessions/:id", authMiddleware, sessionHandler.GetSession)
	h.PUT("/api/sessions/:id", authMiddleware, sessionHandler.UpdateSession)
	h.DELETE("/api/sessions/:id", authMiddleware, sessionHandler.DeleteSession)

	h.POST("/api/messages", authMiddleware, messageHandler.CreateMessage)
	h.GET("/api/sessions/:id/messages", authMiddleware, messageHandler.ListMessages)

	h.POST("/api/chat", authMiddleware, chatHandler.Chat)
	h.POST("/api/chat/stream", authMiddleware, chatHandler.ChatStream)
	h.POST("/api/chat/async", authMiddleware, asyncChatHandler.Submit)
	h.GET("/api/chat/jobs/:id", authMiddleware, asyncChatHandler.GetJob)
	h.POST("/v1/chat/completions", authMiddleware, chatHandler.OpenAIChatCompletions)

	h.POST("/api/knowledge/docs", authMiddleware, knowledgeHandler.CreateDoc)
	h.GET("/api/knowledge/docs", authMiddleware, knowledgeHandler.ListDocs)
	h.GET("/api/knowledge/docs/:id", authMiddleware, knowledgeHandler.GetDoc)
	h.DELETE("/api/knowledge/docs/:id", authMiddleware, knowledgeHandler.DeleteDoc)
	h.POST("/api/chat/knowledge", authMiddleware, knowledgeHandler.ChatWithKnowledge)

	cozeMiddleware := middleware.CozeAPIKey(cfg.CozeAPIKey)
	h.POST("/api/coze/campus/search", cozeMiddleware, cozeHandler.CampusSearch)
	h.POST("/api/coze/campus/ask", cozeMiddleware, cozeHandler.CampusAsk)
	h.POST("/api/coze/events", cozeMiddleware, cozeHandler.Event)
}
