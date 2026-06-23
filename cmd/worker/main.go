package main

import (
	"context"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go-agent-chat-server/internal/agent"
	"go-agent-chat-server/internal/cache"
	"go-agent-chat-server/internal/config"
	"go-agent-chat-server/internal/llm"
	_ "go-agent-chat-server/internal/metrics"
	"go-agent-chat-server/internal/queue"
	"go-agent-chat-server/internal/service"
	"go-agent-chat-server/internal/store"

	"github.com/rabbitmq/amqp091-go"
)

func main() {
	cfg := config.Load()

	if err := store.InitMySQL(cfg.MySQLDSN); err != nil {
		log.Fatalf("init mysql failed: %v", err)
	}
	if err := cache.InitRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB); err != nil {
		log.Fatalf("init redis failed: %v", err)
	}
	if !cfg.AsyncQueueEnabled {
		log.Fatal("ASYNC_QUEUE_ENABLED must be true for worker")
	}

	taskQueue, err := queue.NewRabbitMQChatTaskQueue(queue.RabbitMQTaskConfig{
		URL:        cfg.RabbitMQURL,
		Exchange:   cfg.RabbitMQTaskExchange,
		RoutingKey: cfg.RabbitMQTaskRoutingKey,
		Queue:      cfg.RabbitMQTaskQueue,
		BindingKey: cfg.RabbitMQTaskBindingKey,
	})
	if err != nil {
		log.Fatalf("init rabbitmq chat task queue failed: %v", err)
	}
	defer taskQueue.Close()

	eventPublisher := queue.Publisher(queue.NewNoopPublisher())
	if cfg.MQEnabled {
		rabbitPublisher, err := queue.NewRabbitMQPublisher(queue.RabbitMQConfig{
			URL:        cfg.RabbitMQURL,
			Exchange:   cfg.RabbitMQExchange,
			RoutingKey: cfg.RabbitMQRoutingKey,
			Queue:      cfg.RabbitMQQueue,
			BindingKey: cfg.RabbitMQBindingKey,
		})
		if err != nil {
			log.Fatalf("init rabbitmq event publisher failed: %v", err)
		}
		eventPublisher = rabbitPublisher
		defer eventPublisher.Close()
	}

	if cfg.DebugAddr != "" {
		go func() {
			log.Printf("worker debug server listening on %s, endpoints: /debug/vars /debug/pprof", cfg.DebugAddr)
			if err := http.ListenAndServe(cfg.DebugAddr, nil); err != nil {
				log.Printf("worker debug server stopped: %v", err)
			}
		}()
	}

	messageRepo := store.NewMessageRepo(store.DB)
	sessionRepo := store.NewSessionRepo(store.DB)
	jobRepo := store.NewChatJobRepo(store.DB)
	messageService := service.NewMessageService(messageRepo, sessionRepo)

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
	asyncChatService := service.NewAsyncChatService(jobRepo, chatService, taskQueue, cfg.ChatJobMaxRetry)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	deliveries, err := taskQueue.ConsumeChatTasks(ctx, cfg.WorkerConcurrency)
	if err != nil {
		log.Fatalf("consume chat tasks failed: %v", err)
	}

	if cfg.WorkerConcurrency <= 0 {
		cfg.WorkerConcurrency = 1
	}

	log.Printf("chat worker started, concurrency=%d max_retry=%d", cfg.WorkerConcurrency, cfg.ChatJobMaxRetry)

	sem := make(chan struct{}, cfg.WorkerConcurrency)
	var wg sync.WaitGroup //等待所有任务结束
	for {
		select {
		case <-ctx.Done():
			log.Println("worker shutting down, waiting in-flight tasks")
			wg.Wait()
			return
		case delivery, ok := <-deliveries:
			if !ok {
				log.Println("rabbitmq delivery channel closed")
				wg.Wait()
				return
			}
			sem <- struct{}{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				handleDelivery(ctx, asyncChatService, delivery) //业务处理
			}()
		}
	}
}

func handleDelivery(ctx context.Context, asyncChatService *service.AsyncChatService, delivery amqp091.Delivery) {
	task, err := queue.DecodeChatTask(delivery.Body) //解析消息体
	if err != nil {
		log.Printf("decode chat task failed: %v", err)
		_ = delivery.Ack(false)
		return
	}

	retryable, err := asyncChatService.ProcessTask(ctx, task)
	if err != nil {
		log.Printf("process chat task failed: job_id=%s retryable=%v err=%v", task.JobID, retryable, err)
		if retryable {
			_ = delivery.Nack(false, true)
			return
		}
		_ = delivery.Ack(false)
		return
	}

	_ = delivery.Ack(false)
}
