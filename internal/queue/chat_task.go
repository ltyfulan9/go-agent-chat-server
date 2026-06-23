package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go-agent-chat-server/internal/metrics"

	"github.com/rabbitmq/amqp091-go"
)

type ChatTask struct { //任务消息体结构
	JobID     string    `json:"job_id"`
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	Model     string    `json:"model"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatTaskPublisher interface {
	PublishChatTask(ctx context.Context, task ChatTask) error
	Close() error
}

type ChatTaskConsumer interface {
	ConsumeChatTasks(ctx context.Context, prefetch int) (<-chan amqp091.Delivery, error)
	Close() error
}

type NoopChatTaskPublisher struct{}

func NewNoopChatTaskPublisher() *NoopChatTaskPublisher { return &NoopChatTaskPublisher{} }

func (p *NoopChatTaskPublisher) PublishChatTask(ctx context.Context, task ChatTask) error {
	return fmt.Errorf("async chat task queue is disabled")
}

func (p *NoopChatTaskPublisher) Close() error { return nil }

type RabbitMQTaskConfig struct {
	URL        string
	Exchange   string
	RoutingKey string
	Queue      string
	BindingKey string
}

type RabbitMQChatTaskQueue struct {
	conn       *amqp091.Connection
	channel    *amqp091.Channel
	exchange   string
	routingKey string
	queue      string
	confirms   <-chan amqp091.Confirmation
	returns    <-chan amqp091.Return
	mu         sync.Mutex
}

func NewRabbitMQChatTaskQueue(cfg RabbitMQTaskConfig) (*RabbitMQChatTaskQueue, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("rabbitmq url is required")
	}
	if cfg.Exchange == "" {
		cfg.Exchange = "chat.tasks"
	}
	if cfg.RoutingKey == "" {
		cfg.RoutingKey = "chat.task.created"
	}
	if cfg.Queue == "" {
		cfg.Queue = "chat.tasks.worker"
	}
	if cfg.BindingKey == "" {
		cfg.BindingKey = "chat.task.*"
	}

	conn, err := amqp091.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("connect rabbitmq task queue failed: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq task channel failed: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("enable rabbitmq task publisher confirm failed: %w", err)
	}

	if err := ch.ExchangeDeclare(cfg.Exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare rabbitmq task exchange failed: %w", err)
	}

	if _, err := ch.QueueDeclare(cfg.Queue, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare rabbitmq task queue failed: %w", err)
	}

	if err := ch.QueueBind(cfg.Queue, cfg.BindingKey, cfg.Exchange, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("bind rabbitmq task queue failed: %w", err)
	}

	return &RabbitMQChatTaskQueue{
		conn:       conn,
		channel:    ch,
		exchange:   cfg.Exchange,
		routingKey: cfg.RoutingKey,
		queue:      cfg.Queue,
		confirms:   ch.NotifyPublish(make(chan amqp091.Confirmation, 1)),
		returns:    ch.NotifyReturn(make(chan amqp091.Return, 1)),
	}, nil
}

func (q *RabbitMQChatTaskQueue) PublishChatTask(ctx context.Context, task ChatTask) error {
	if q == nil || q.channel == nil {
		return fmt.Errorf("rabbitmq task queue is not initialized")
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	body, err := json.Marshal(task)
	if err != nil {
		metrics.ChatJobPublishFailedTotal.Add(1)
		return fmt.Errorf("encode chat task failed: %w", err)
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	metrics.ChatJobPublishedTotal.Add(1)

	if err := q.channel.PublishWithContext(ctx, q.exchange, q.routingKey, true, false, amqp091.Publishing{
		ContentType:   "application/json",
		DeliveryMode:  amqp091.Persistent,
		Timestamp:     task.CreatedAt,
		Body:          body,
		CorrelationId: task.JobID,
	}); err != nil {
		metrics.ChatJobPublishFailedTotal.Add(1)
		return fmt.Errorf("publish chat task failed: %w", err)
	}

	select {
	case returned := <-q.returns:
		metrics.ChatJobPublishFailedTotal.Add(1)
		return fmt.Errorf("chat task unroutable: reply_code=%d reply_text=%s exchange=%s routing_key=%s", returned.ReplyCode, returned.ReplyText, returned.Exchange, returned.RoutingKey)
	default:
	}

	select {
	case confirm, ok := <-q.confirms:
		if !ok {
			metrics.ChatJobPublishFailedTotal.Add(1)
			return fmt.Errorf("rabbitmq task confirm channel closed")
		}
		if !confirm.Ack {
			metrics.ChatJobPublishFailedTotal.Add(1)
			return fmt.Errorf("rabbitmq task publish not acknowledged")
		}
		return nil
	case <-ctx.Done():
		metrics.ChatJobPublishFailedTotal.Add(1)
		return ctx.Err()
	}
}

func (q *RabbitMQChatTaskQueue) ConsumeChatTasks(ctx context.Context, prefetch int) (<-chan amqp091.Delivery, error) {
	if q == nil || q.channel == nil {
		return nil, fmt.Errorf("rabbitmq task queue is not initialized")
	}
	if prefetch <= 0 {
		prefetch = 1
	}
	if err := q.channel.Qos(prefetch, 0, false); err != nil {
		return nil, fmt.Errorf("set rabbitmq task qos failed: %w", err)
	}
	return q.channel.ConsumeWithContext(ctx, q.queue, "", false, false, false, false, nil)
}

func (q *RabbitMQChatTaskQueue) Close() error {
	if q == nil {
		return nil
	}
	if q.channel != nil {
		_ = q.channel.Close()
	}
	if q.conn != nil {
		return q.conn.Close()
	}
	return nil
}

func DecodeChatTask(body []byte) (ChatTask, error) {
	var task ChatTask
	if err := json.Unmarshal(body, &task); err != nil {
		return ChatTask{}, err
	}
	if task.JobID == "" {
		return ChatTask{}, fmt.Errorf("chat task job_id is required")
	}
	return task, nil
}
