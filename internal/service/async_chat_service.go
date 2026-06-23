package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"go-agent-chat-server/internal/metrics"
	"go-agent-chat-server/internal/model"
	"go-agent-chat-server/internal/pkg/idgen"
	"go-agent-chat-server/internal/queue"
	"go-agent-chat-server/internal/store"
)

var ErrAsyncQueueDisabled = errors.New("async chat queue is disabled")

type AsyncChatService struct {
	jobRepo       *store.ChatJobRepo
	chatService   *ChatService
	taskPublisher queue.ChatTaskPublisher
	maxRetry      int //任务失败后最多重试几次
}

func NewAsyncChatService(jobRepo *store.ChatJobRepo, chatService *ChatService, taskPublisher queue.ChatTaskPublisher, maxRetry int) *AsyncChatService {
	if taskPublisher == nil {
		taskPublisher = queue.NewNoopChatTaskPublisher()
	}
	if maxRetry <= 0 {
		maxRetry = 3
	}
	return &AsyncChatService{
		jobRepo:       jobRepo,
		chatService:   chatService,
		taskPublisher: taskPublisher,
		maxRetry:      maxRetry,
	}
}

func (s *AsyncChatService) Submit(ctx context.Context, userID, sessionID, modelName, message string) (model.ChatJob, error) {
	if userID == "" {
		return model.ChatJob{}, errors.New("user_id is required")
	}
	if sessionID == "" {
		return model.ChatJob{}, errors.New("session_id is required")
	}
	if strings.TrimSpace(message) == "" {
		return model.ChatJob{}, errors.New("message is required")
	}
	if s.taskPublisher == nil {
		return model.ChatJob{}, ErrAsyncQueueDisabled
	}
	if _, disabled := s.taskPublisher.(*queue.NoopChatTaskPublisher); disabled {
		return model.ChatJob{}, ErrAsyncQueueDisabled
	}

	if err := s.chatService.checkUserLLMRateLimit(ctx, userID); err != nil {
		return model.ChatJob{}, err
	}

	userMsg, err := s.chatService.messageService.CreateMessage(ctx, userID, sessionID, "user", message)
	if err != nil {
		return model.ChatJob{}, err
	}

	job := model.ChatJob{ //创建一个异步任务对象
		ID:            idgen.NewID(), //任务ID
		UserID:        userID,
		SessionID:     sessionID,
		Model:         modelName,
		Status:        model.ChatJobStatusPending,
		RequestText:   message,
		UserMessageID: userMsg.ID, //刚才保存的user消息ID
	}
	if err := s.jobRepo.Create(&job); err != nil {
		return model.ChatJob{}, err //写入chat_jobs表
	}
	metrics.ChatJobCreatedTotal.Add(1)

	publishCtx, cancel := context.WithTimeout(ctx, 2*time.Second) //发布任务到RabbitMQ
	defer cancel()
	if err := s.taskPublisher.PublishChatTask(publishCtx, queue.ChatTask{
		JobID:     job.ID,
		UserID:    userID,
		SessionID: sessionID,
		Model:     modelName,
		Message:   message,
		CreatedAt: time.Now(),
	}); err != nil {
		// 如果发布任务失败，将任务标记为失败
		_ = s.jobRepo.MarkFailed(job.ID, err.Error(), 0)
		return model.ChatJob{}, err
	}

	return job, nil
}

// 查询任务状态
func (s *AsyncChatService) GetJob(ctx context.Context, userID, jobID string) (model.ChatJob, error) {
	if userID == "" {
		return model.ChatJob{}, errors.New("user_id is required")
	}
	if jobID == "" {
		return model.ChatJob{}, errors.New("job_id is required")
	}
	return s.jobRepo.GetByIDAndUserID(jobID, userID)
}

func (s *AsyncChatService) ProcessTask(ctx context.Context, task queue.ChatTask) (bool, error) {
	job, err := s.jobRepo.GetByID(task.JobID)
	if err != nil {
		return false, err
	}
	if job.Status == model.ChatJobStatusSuccess {
		return false, nil
	}
	if job.RetryCount >= s.maxRetry && job.Status == model.ChatJobStatusFailed {
		return false, nil
	}

	if err := s.jobRepo.MarkRunning(job.ID); err != nil {
		return true, err
	}
	metrics.ChatJobRunningTotal.Add(1)
	if !job.CreatedAt.IsZero() {
		metrics.ChatJobQueueWaitMsTotal.Add(metrics.DurationMs(time.Since(job.CreatedAt)))
	}

	result, err := s.chatService.GenerateAnswerFromHistory(ctx, job.UserID, job.SessionID, job.Model, job.RequestText)
	if err != nil {
		nextRetry := job.RetryCount + 1
		if nextRetry >= s.maxRetry {
			_ = s.jobRepo.MarkFailed(job.ID, err.Error(), nextRetry)
			metrics.ChatJobFailedTotal.Add(1)
			return false, err
		}
		_ = s.jobRepo.MarkPendingForRetry(job.ID, err.Error(), nextRetry)
		metrics.ChatJobRetriedTotal.Add(1)
		return true, err
	}

	if err := s.jobRepo.MarkSuccess(job.ID, result.Answer, result.AssistantMessageID); err != nil {
		return true, err
	}
	metrics.ChatJobSuccessTotal.Add(1)
	return false, nil
}
