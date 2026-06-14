package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerAddr string

	MySQLDSN string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	OllamaBaseURL string
	DefaultModel  string

	MaxConcurrentLLM         int
	UserLLMRateLimit         int
	UserLLMRateWindowSeconds int

	JWTSecret      string
	JWTExpireHours int

	MQEnabled          bool
	RabbitMQURL        string
	RabbitMQExchange   string
	RabbitMQRoutingKey string
	RabbitMQQueue      string
	RabbitMQBindingKey string

	CozeAPIKey        string
	CozeDefaultUserID string
}

func Load() *Config {
	_ = godotenv.Load() // 读取根目录下的 .env 文件，加载到环境变量中；不会覆盖已存在的环境变量

	return &Config{
		ServerAddr: getEnv("SERVER_ADDR", ":8080"),

		MySQLDSN: getEnv("MYSQL_DSN", "root:password@tcp(localhost:3306)/go_chat?charset=utf8mb4&parseTime=True&loc=Local"),

		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvAsInt("REDIS_DB", 0),

		OllamaBaseURL: getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		DefaultModel:  getEnv("DEFAULT_MODEL", "qwen2.5:3b"),

		MaxConcurrentLLM:         getEnvAsInt("MAX_CONCURRENT_LLM", 2),
		UserLLMRateLimit:         getEnvAsInt("USER_LLM_RATE_LIMIT", 10),
		UserLLMRateWindowSeconds: getEnvAsInt("USER_LLM_RATE_WINDOW_SECONDS", 60),

		JWTSecret:      getEnv("JWT_SECRET", "dev-secret-change-me"),
		JWTExpireHours: getEnvAsInt("JWT_EXPIRE_HOURS", 24),

		MQEnabled:          getEnvAsBool("MQ_ENABLED", false),
		RabbitMQURL:        getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitMQExchange:   getEnv("RABBITMQ_EXCHANGE", "chat.events"),
		RabbitMQRoutingKey: getEnv("RABBITMQ_ROUTING_KEY", "chat.event"),
		RabbitMQQueue:      getEnv("RABBITMQ_QUEUE", "chat.events.log"),
		RabbitMQBindingKey: getEnv("RABBITMQ_BINDING_KEY", "chat.*"),

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
