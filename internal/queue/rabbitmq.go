package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-agent-chat-server/internal/metrics"

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
	conn       *amqp091.Connection
	channel    *amqp091.Channel
	exchange   string
	routingKey string
	confirms   <-chan amqp091.Confirmation
	returns    <-chan amqp091.Return
	mu         sync.Mutex
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

	conn, err := amqp091.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("connect rabbitmq failed: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel failed: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("enable rabbitmq publisher confirm failed: %w", err)
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
		confirms:   ch.NotifyPublish(make(chan amqp091.Confirmation, 1)),
		returns:    ch.NotifyReturn(make(chan amqp091.Return, 1)),
	}, nil
}

func (p *RabbitMQPublisher) Publish(ctx context.Context, event Event) error {
	if p == nil || p.channel == nil {
		return nil
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	body, err := EncodeEvent(event)
	if err != nil {
		metrics.RabbitMQPublishFailedTotal.Add(1)
		return fmt.Errorf("encode event failed: %w", err)
	}

	routingKey := p.routingKey
	if event.Type != "" {
		routingKey = event.Type
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	metrics.RabbitMQPublishTotal.Add(1)

	if err := p.channel.PublishWithContext(
		ctx,
		p.exchange,
		routingKey,
		true,
		false,
		amqp091.Publishing{
			ContentType:   "application/json",
			DeliveryMode:  amqp091.Persistent,
			Timestamp:     event.CreatedAt,
			Body:          body,
			CorrelationId: event.ID,
		},
	); err != nil {
		metrics.RabbitMQPublishFailedTotal.Add(1)
		return fmt.Errorf("publish rabbitmq event failed: %w", err)
	}

	select {
	case returned := <-p.returns:
		metrics.RabbitMQPublishFailedTotal.Add(1)
		return fmt.Errorf("rabbitmq event unroutable: reply_code=%d reply_text=%s exchange=%s routing_key=%s", returned.ReplyCode, returned.ReplyText, returned.Exchange, returned.RoutingKey)
	default:
	}

	select {
	case confirm, ok := <-p.confirms:
		if !ok {
			metrics.RabbitMQPublishFailedTotal.Add(1)
			return fmt.Errorf("rabbitmq confirm channel closed")
		}
		if !confirm.Ack {
			metrics.RabbitMQPublishFailedTotal.Add(1)
			return fmt.Errorf("rabbitmq publish not acknowledged")
		}
		return nil
	case <-ctx.Done():
		metrics.RabbitMQPublishFailedTotal.Add(1)
		return ctx.Err()
	}
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
