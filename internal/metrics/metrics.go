package metrics

import (
	"expvar"
	"sync/atomic"
	"time"
)

var (
	HTTPRequestsTotal = expvar.NewInt("http_requests_total")
	HTTPErrorsTotal   = expvar.NewInt("http_errors_total")

	MessageCacheHitTotal   = expvar.NewInt("message_cache_hit_total")
	MessageCacheMissTotal  = expvar.NewInt("message_cache_miss_total")
	MessageCacheErrorTotal = expvar.NewInt("message_cache_error_total")

	IPRateLimitCheckedTotal = expvar.NewInt("ip_rate_limit_checked_total")
	IPRateLimitBlockedTotal = expvar.NewInt("ip_rate_limit_blocked_total")
	IPRateLimitErrorTotal   = expvar.NewInt("ip_rate_limit_error_total")

	UserLLMRateLimitCheckedTotal = expvar.NewInt("user_llm_rate_limit_checked_total")
	UserLLMRateLimitBlockedTotal = expvar.NewInt("user_llm_rate_limit_blocked_total")
	UserLLMRateLimitErrorTotal   = expvar.NewInt("user_llm_rate_limit_error_total")

	LLMAcquireTotal         = expvar.NewInt("llm_acquire_total")
	LLMAcquireRejectedTotal = expvar.NewInt("llm_acquire_rejected_total")
	LLMAcquireWaitMsTotal   = expvar.NewInt("llm_acquire_wait_ms_total")
	LLMInFlight             = expvar.NewInt("llm_inflight")
	LLMCallTotal            = expvar.NewInt("llm_call_total")
	LLMCallFailedTotal      = expvar.NewInt("llm_call_failed_total")
	LLMCallDurationMsTotal  = expvar.NewInt("llm_call_duration_ms_total")

	SSEConnectionsTotal  = expvar.NewInt("sse_connections_total")
	SSEConnectionsActive = expvar.NewInt("sse_connections_active")
	SSEWriteErrorTotal   = expvar.NewInt("sse_write_error_total")

	RabbitMQPublishTotal       = expvar.NewInt("rabbitmq_publish_total")
	RabbitMQPublishFailedTotal = expvar.NewInt("rabbitmq_publish_failed_total")

	ChatJobCreatedTotal       = expvar.NewInt("chat_job_created_total")
	ChatJobPublishedTotal     = expvar.NewInt("chat_job_published_total")
	ChatJobPublishFailedTotal = expvar.NewInt("chat_job_publish_failed_total")
	ChatJobRunningTotal       = expvar.NewInt("chat_job_running_total")
	ChatJobSuccessTotal       = expvar.NewInt("chat_job_success_total")
	ChatJobFailedTotal        = expvar.NewInt("chat_job_failed_total")
	ChatJobRetriedTotal       = expvar.NewInt("chat_job_retried_total")
	ChatJobQueueWaitMsTotal   = expvar.NewInt("chat_job_queue_wait_ms_total")
)

var startTime = time.Now()

func init() {
	expvar.Publish("app_uptime_seconds", expvar.Func(func() interface{} {
		return int64(time.Since(startTime).Seconds())
	}))
	expvar.Publish("app_build_info", expvar.Func(func() interface{} {
		return map[string]string{
			"service": "go-agent-chat-server",
			"profile": "observable",
		}
	}))
}

var sseActive int64

func IncSSEActive() {
	atomic.AddInt64(&sseActive, 1)
	SSEConnectionsActive.Add(1)
}

func DecSSEActive() {
	atomic.AddInt64(&sseActive, -1)
	SSEConnectionsActive.Add(-1)
}

func DurationMs(d time.Duration) int64 {
	return d.Milliseconds()
}
