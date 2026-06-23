package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerAddr string
	DebugAddr  string

	MySQLDSN string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	OllamaBaseURL string
	DefaultModel  string

	MaxConcurrentLLM         int
	UserLLMRateLimit         int
	UserLLMRateWindowSeconds int
	IPRateLimit              int
	IPRateWindowSeconds      int

	JWTSecret      string
	JWTExpireHours int

	MQEnabled          bool
	RabbitMQURL        string
	RabbitMQExchange   string
	RabbitMQRoutingKey string
	RabbitMQQueue      string
	RabbitMQBindingKey string

	AsyncQueueEnabled      bool
	RabbitMQTaskExchange   string
	RabbitMQTaskRoutingKey string
	RabbitMQTaskQueue      string
	RabbitMQTaskBindingKey string
	WorkerConcurrency      int
	ChatJobMaxRetry        int

	CozeAPIKey        string
	CozeDefaultUserID string
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		ServerAddr: getEnv("SERVER_ADDR", ":8080"),
		DebugAddr:  getEnv("DEBUG_ADDR", ":6060"),

		MySQLDSN: getEnv("MYSQL_DSN", "root:password@tcp(localhost:3306)/go_chat?charset=utf8mb4&parseTime=True&loc=Local"),

		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvAsInt("REDIS_DB", 0),

		OllamaBaseURL: getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		DefaultModel:  getEnv("DEFAULT_MODEL", "qwen2.5:3b"),

		MaxConcurrentLLM:         getEnvAsInt("MAX_CONCURRENT_LLM", 2),
		UserLLMRateLimit:         getEnvAsInt("USER_LLM_RATE_LIMIT", 10),
		UserLLMRateWindowSeconds: getEnvAsInt("USER_LLM_RATE_WINDOW_SECONDS", 60),
		IPRateLimit:              getEnvAsInt("IP_RATE_LIMIT", 60),
		IPRateWindowSeconds:      getEnvAsInt("IP_RATE_WINDOW_SECONDS", 60),

		JWTSecret:      getEnv("JWT_SECRET", "dev-secret-change-me"),
		JWTExpireHours: getEnvAsInt("JWT_EXPIRE_HOURS", 24),

		MQEnabled:          getEnvAsBool("MQ_ENABLED", false),
		RabbitMQURL:        getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitMQExchange:   getEnv("RABBITMQ_EXCHANGE", "chat.events"),
		RabbitMQRoutingKey: getEnv("RABBITMQ_ROUTING_KEY", "chat.event"),
		RabbitMQQueue:      getEnv("RABBITMQ_QUEUE", "chat.events.log"),
		RabbitMQBindingKey: getEnv("RABBITMQ_BINDING_KEY", "chat.*"),

		AsyncQueueEnabled:      getEnvAsBool("ASYNC_QUEUE_ENABLED", false),
		RabbitMQTaskExchange:   getEnv("RABBITMQ_TASK_EXCHANGE", "chat.tasks"),
		RabbitMQTaskRoutingKey: getEnv("RABBITMQ_TASK_ROUTING_KEY", "chat.task.created"),
		RabbitMQTaskQueue:      getEnv("RABBITMQ_TASK_QUEUE", "chat.tasks.worker"),
		RabbitMQTaskBindingKey: getEnv("RABBITMQ_TASK_BINDING_KEY", "chat.task.*"),
		WorkerConcurrency:      getEnvAsInt("WORKER_CONCURRENCY", 2),
		ChatJobMaxRetry:        getEnvAsInt("CHAT_JOB_MAX_RETRY", 3),

		CozeAPIKey:        getEnv("COZE_API_KEY", ""),
		CozeDefaultUserID: getEnv("COZE_DEFAULT_USER_ID", "coze-default-user"),
	}
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}

	return boolValue
}
