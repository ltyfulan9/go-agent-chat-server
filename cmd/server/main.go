package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"go-agent-chat-server/internal/cache"
	"go-agent-chat-server/internal/config"
	_ "go-agent-chat-server/internal/metrics"
	"go-agent-chat-server/internal/queue"
	"go-agent-chat-server/internal/router"
	"go-agent-chat-server/internal/store"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func main() {
	cfg := config.Load()

	if err := store.InitMySQL(cfg.MySQLDSN); err != nil {
		log.Fatalf("init mysql failed: %v", err)
	}

	if err := cache.InitRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB); err != nil {
		log.Fatalf("init redis failed: %v", err)
	}

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
			log.Fatalf("init rabbitmq failed: %v", err)
		}
		eventPublisher = rabbitPublisher
		defer eventPublisher.Close()
		log.Println("rabbitmq event publisher enabled")
	}

	chatTaskPublisher := queue.ChatTaskPublisher(queue.NewNoopChatTaskPublisher())
	//先用一个空实现兜底，如果不开异步队列，那么调用异步发布接口时不会真的投递任务
	if cfg.AsyncQueueEnabled {
		rabbitTaskQueue, err := queue.NewRabbitMQChatTaskQueue(queue.RabbitMQTaskConfig{
			URL:        cfg.RabbitMQURL,
			Exchange:   cfg.RabbitMQTaskExchange,
			RoutingKey: cfg.RabbitMQTaskRoutingKey,
			Queue:      cfg.RabbitMQTaskQueue,
			BindingKey: cfg.RabbitMQTaskBindingKey,
		})
		if err != nil {
			log.Fatalf("init rabbitmq chat task queue failed: %v", err)
		}
		chatTaskPublisher = rabbitTaskQueue
		defer chatTaskPublisher.Close()
		log.Println("rabbitmq async chat task publisher enabled")
	}

	if cfg.DebugAddr != "" {
		go func() {
			log.Printf("debug server listening on %s, endpoints: /debug/vars /debug/pprof", cfg.DebugAddr)
			if err := http.ListenAndServe(cfg.DebugAddr, nil); err != nil {
				log.Printf("debug server stopped: %v", err)
			}
		}()
	}

	h := server.Default(
		server.WithHostPorts(cfg.ServerAddr),
	)

	router.Register(h, cfg, eventPublisher, chatTaskPublisher)

	h.Spin()
}
