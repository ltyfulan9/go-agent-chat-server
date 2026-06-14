# go-agent-chat-server

A Go-based multi-user AI chat backend built with Hertz, MySQL, Redis, RabbitMQ, Ollama, Eino, SSE, JWT, lightweight Tool Calling, and a mini keyword-based knowledge base.

## Features

- CloudWeGo Hertz REST API
- User register/login with JWT authentication
- Snowflake-style string IDs
- Session/message CRUD
- Paginated session and message list APIs
- Transactional session deletion: delete messages + session in one MySQL transaction
- MySQL persistence with GORM and composite indexes
- Redis IP rate limiting with Lua script
- Redis user-level LLM call rate limiting
- Redis cache for full session message history
- RabbitMQ async event publishing for chat completion events
- Request ID, access logging, and panic recovery middleware
- Ollama local model integration
- Eino-based non-stream chat orchestration
- SSE streaming chat API
- OpenAI-compatible `/v1/chat/completions` API
- LLM concurrency control with Go channel semaphore
- Lightweight Tool Calling: calculator, time, and session-history search
- Mini Knowledge Base: MySQL keyword retrieval + context injection
- Coze integration APIs for campus assistant workflows and plugins
- Dockerfile, Docker Compose, Makefile
- Optional MQ-driven event flow for resume-friendly async architecture
- Starter unit tests for ID generator, JWT, pagination, and calculator tool

## Tech Stack

- Go
- CloudWeGo Hertz
- GORM
- MySQL
- Redis / go-redis v9
- RabbitMQ / amqp091-go
- Redis Lua Script
- Ollama
- Eino
- SSE
- JWT HMAC-SHA256
- bcrypt
- godotenv
- Docker Compose
- RabbitMQ Management UI
- Coze HTTP node / custom plugin integration

## Project Structure

```text
go-agent-chat-server/
├── cmd/server/main.go
├── internal/
│   ├── agent/              # Eino runner
│   ├── api/                # HTTP handlers and response helpers
│   ├── cache/              # Redis client, message cache, LLM rate limit
│   ├── config/             # env config loader
│   ├── ecode/              # unified error codes
│   ├── llm/                # Ollama client
│   ├── middleware/         # auth, rate limit, logger, recovery, request_id
│   ├── model/              # GORM models
│   ├── pkg/                # idgen, jwtutil
│   ├── queue/              # RabbitMQ / Noop event publisher
│   ├── router/             # route registration
│   ├── service/            # business logic
│   ├── store/              # repositories
│   └── tool/               # lightweight Tool Calling
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── .env.example
├── go.mod
└── README.md
```

## API List

### Auth

```http
POST /api/auth/register
POST /api/auth/login
```

### Sessions

```http
POST   /api/sessions
GET    /api/sessions?page=1&page_size=20
GET    /api/sessions/:id
PUT    /api/sessions/:id
DELETE /api/sessions/:id
```

### Messages

```http
POST /api/messages
GET  /api/sessions/:id/messages?page=1&page_size=30
```

### Chat

```http
POST /api/chat
POST /api/chat/stream
POST /v1/chat/completions
```

### Mini Knowledge Base

```http
POST   /api/knowledge/docs
GET    /api/knowledge/docs?page=1&page_size=20
GET    /api/knowledge/docs/:id
DELETE /api/knowledge/docs/:id
POST   /api/chat/knowledge
```

### Coze Integration APIs

These APIs are designed for Coze HTTP nodes or custom plugins. If `COZE_API_KEY` is configured, call them with `X-Coze-API-Key`, `X-API-Key`, or `Authorization: Bearer <key>`.

```http
POST /api/coze/campus/search
POST /api/coze/campus/ask
POST /api/coze/events
```

All APIs except health/register/login and `/api/coze/*` require:

```http
Authorization: Bearer your_token
```

## Async MQ Events

When `MQ_ENABLED=true`, the server publishes chat lifecycle events to RabbitMQ after normal and streaming chat requests finish.

Default exchange and routing keys:

```text
exchange: chat.events
routing key: chat.completed / chat.stream.completed
queue: chat.events.log
binding: chat.*
```

Event payload example:

```json
{
  "id": "snowflake_id",
  "type": "chat.completed",
  "user_id": "user_id",
  "session_id": "session_id",
  "created_at": "2026-01-01T00:00:00Z",
  "payload": {
    "model": "qwen2.5:3b",
    "user_message_length": 32,
    "answer_length": 128,
    "assistant_message_id": "message_id"
  }
}
```

## Local Setup

### 1. Configure `.env`

```bash
cp .env.example .env
```

Update your local MySQL password in `.env`:

```env
MYSQL_DSN=root:your_password@tcp(localhost:3306)/go_chat?charset=utf8mb4&parseTime=True&loc=Local
```

### 2. Start MySQL and Redis

You can start MySQL locally, then create the database:

```sql
CREATE DATABASE go_chat DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

Start Redis:

```bash
docker run -d --name go-chat-redis -p 6379:6379 redis:7
```

### 3. Start Ollama

```bash
ollama run qwen2.5:3b
```

### 4. Run server

```bash
make run
```

## Docker Compose

Docker Compose starts MySQL, Redis, RabbitMQ, and the Go server. Ollama is expected to run on your host machine.

```bash
make docker-up
make docker-logs
```

Stop services:

```bash
make docker-down
```

## Example Requests

### Register

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"123456"}'
```

### Create session

```bash
curl -X POST http://localhost:8080/api/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"first chat"}'
```

### Paginated sessions

```bash
curl "http://localhost:8080/api/sessions?page=1&page_size=20" \
  -H "Authorization: Bearer $TOKEN"
```

### Normal chat

```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"session_id":"SESSION_ID","model":"qwen2.5:3b","message":"帮我计算 1 + 2 * 3"}'
```

### SSE stream chat

```bash
curl -N -X POST http://localhost:8080/api/chat/stream \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"session_id":"SESSION_ID","model":"qwen2.5:3b","message":"Explain Go context"}'
```

### OpenAI-compatible API

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen2.5:3b","messages":[{"role":"user","content":"hello"}],"stream":false}'
```

### Create knowledge document

```bash
curl -X POST http://localhost:8080/api/knowledge/docs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Go context","content":"context is used to carry deadlines, cancellation signals, and request-scoped values."}'
```

### Chat with knowledge

```bash
curl -X POST http://localhost:8080/api/chat/knowledge \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"session_id":"SESSION_ID","model":"qwen2.5:3b","message":"Go context 是干什么的？","top_k":5}'
```


### Coze campus ask

```bash
curl -X POST http://localhost:8080/api/coze/campus/ask \
  -H "X-Coze-API-Key: $COZE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"user_id":"USER_ID","coze_user_id":"coze-user-1","query":"云南大学大创怎么报名？","intent":"办事通","history_summary":"用户正在咨询大创流程","top_k":5}'
```

Response contains `answer`, `sources`, `matched_docs`, and `need_web`, so Coze can decide whether to continue web search or directly polish the answer.

### Coze campus search

```bash
curl -X POST http://localhost:8080/api/coze/campus/search \
  -H "X-Coze-API-Key: $COZE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"user_id":"USER_ID","coze_user_id":"coze-user-1","query":"奖学金申请材料","intent":"学业通","top_k":5}'
```

### Coze event log

```bash
curl -X POST http://localhost:8080/api/coze/events \
  -H "X-Coze-API-Key: $COZE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"type":"coze.intent.detected","user_id":"USER_ID","coze_user_id":"coze-user-1","payload":{"intent":"办事通"}}'
```

## Makefile

```bash
make run
make test
make tidy
make fmt
make docker-up
make docker-down
make docker-logs
```

## Notes

- The knowledge base is intentionally lightweight. It uses MySQL keyword search first, not vector search. This keeps the project focused on Go backend engineering while still demonstrating RAG-style context injection.
- The Tool Calling implementation is heuristic-based and lightweight. It is meant to show Agent backend design concepts without pulling in a heavy framework.
- Coze integration APIs let a Coze bot delegate campus search, answer generation, and event logging to this Go backend through HTTP nodes or custom plugins.
- For production, replace `.env` secrets, add migration tooling, improve observability, and consider pgvector/Milvus + embedding + rerank for a stronger RAG module.
