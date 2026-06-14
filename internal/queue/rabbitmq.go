package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type RabbitMQConfig struct {
	URL        string
	Exchange   string
	RoutingKey string
	Queue      string
	BindingKey string
}

type RabbitMQPublisher struct {
	conn       *amqp091.Connection //MQ TCP连接，支持自动重连和连接池等功能
	channel    *amqp091.Channel    //AMQP 0-9-1协议的信道，所有的操作都在这个信道上进行
	exchange   string
	routingKey string
}

func NewRabbitMQPublisher(cfg RabbitMQConfig) (*RabbitMQPublisher, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("rabbitmq url is required")
	}
	if cfg.Exchange == "" {
		cfg.Exchange = "chat.events"
	}
	if cfg.RoutingKey == "" {
		cfg.RoutingKey = "chat.event"
	}
	if cfg.Queue == "" {
		cfg.Queue = "chat.events.log"
	}
	if cfg.BindingKey == "" {
		cfg.BindingKey = "chat.*"
	}

	conn, err := amqp091.Dial(cfg.URL) //连接RabbitMQ
	if err != nil {
		return nil, fmt.Errorf("connect rabbitmq failed: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel failed: %w", err)
	}

	if err := ch.ExchangeDeclare(
		cfg.Exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare rabbitmq exchange failed: %w", err)
	}

	if cfg.Queue != "" {
		if _, err := ch.QueueDeclare(
			cfg.Queue,
			true,
			false,
			false,
			false,
			nil,
		); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, fmt.Errorf("declare rabbitmq queue failed: %w", err)
		}

		if err := ch.QueueBind(
			cfg.Queue,
			cfg.BindingKey,
			cfg.Exchange,
			false,
			nil,
		); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, fmt.Errorf("bind rabbitmq queue failed: %w", err)
		}
	}

	return &RabbitMQPublisher{
		conn:       conn,
		channel:    ch,
		exchange:   cfg.Exchange,
		routingKey: cfg.RoutingKey,
	}, nil
}

func (p *RabbitMQPublisher) Publish(ctx context.Context, event Event) error {
	if p == nil || p.channel == nil {
		return fmt.Errorf("rabbitmq publisher is not initialized")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	body, err := EncodeEvent(event)
	if err != nil {
		return fmt.Errorf("encode event failed: %w", err)
	}

	routingKey := p.routingKey
	if event.Type != "" {
		routingKey = event.Type
	}

	return p.channel.PublishWithContext(
		ctx,
		p.exchange,
		routingKey,
		false,
		false,
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent,
			Timestamp:    event.CreatedAt,
			Body:         body,
		},
	)
}

func (p *RabbitMQPublisher) Close() error {
	if p == nil {
		return nil
	}
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}
