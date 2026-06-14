package main

import (
	"log"

	"go-agent-chat-server/internal/cache"
	"go-agent-chat-server/internal/config"
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

	h := server.Default(
		server.WithHostPorts(cfg.ServerAddr),
	)

	router.Register(h, cfg, eventPublisher)

	h.Spin()
}
