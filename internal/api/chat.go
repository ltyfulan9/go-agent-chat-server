package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-agent-chat-server/internal/ecode"
	"go-agent-chat-server/internal/model"
	"io"
	"time"

	"go-agent-chat-server/internal/service"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type ChatRequest struct {
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
	Message   string `json:"message"`
}

type OpenAIChatCompletionRequest struct {
	Model    string          `json:"model"`
	Messages []OpenAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIChatCompletionResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message,omitempty"`
	Delta        OpenAIMessage `json:"delta,omitempty"`
	FinishReason string        `json:"finish_reason,omitempty"`
}

type ChatHandler struct {
	chatService *service.ChatService
}

func NewChatHandler(chatService *service.ChatService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
	}
}

func (h *ChatHandler) Chat(ctx context.Context, c *app.RequestContext) {
	var req ChatRequest

	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid request"))
		return
	}

	userID := CurrentUserID(c)
	result, err := h.chatService.Chat(ctx, userID, req.SessionID, req.Model, req.Message)
	if err != nil {
		writeChatError(c, err)
		return
	}

	c.JSON(consts.StatusOK, Success(result))
}

func (h *ChatHandler) ChatStream(ctx context.Context, c *app.RequestContext) {
	var req ChatRequest

	body := c.Request.Body()
	if len(body) == 0 {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "empty request body"))
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid json: "+err.Error()))
		return
	}

	if req.SessionID == "" || req.Message == "" {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "session_id and message are required"))
		return
	}

	userID := CurrentUserID(c)
	reader, writer := io.Pipe()

	c.SetStatusCode(consts.StatusOK)
	c.Response.Header.Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Response.Header.Set("Cache-Control", "no-cache")
	c.Response.Header.Set("Connection", "keep-alive")

	c.SetBodyStream(reader, -1)

	go func() {
		defer writer.Close()

		_, err := h.chatService.StreamChat(ctx, userID, req.SessionID, req.Model, req.Message, func(delta string) error {
			return writeSSE(writer, "message", map[string]string{
				"delta": delta,
			})
		})

		if err != nil {
			_ = writeSSE(writer, "error", map[string]string{
				"error": err.Error(),
			})
			return
		}

		_ = writeSSE(writer, "done", map[string]bool{
			"done": true,
		})
	}()
}

func (h *ChatHandler) OpenAIChatCompletions(ctx context.Context, c *app.RequestContext) {
	var req OpenAIChatCompletionRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "invalid request"))
		return
	}
	if len(req.Messages) == 0 {
		c.JSON(consts.StatusBadRequest, Error(ecode.CodeInvalidRequest, "messages are required"))
		return
	}

	messages := make([]model.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, model.Message{Role: m.Role, Content: m.Content})
	}

	userID := CurrentUserID(c)
	if !req.Stream {
		answer, err := h.chatService.ChatCompletion(ctx, userID, req.Model, messages)
		if err != nil {
			writeChatError(c, err)
			return
		}

		c.JSON(consts.StatusOK, OpenAIChatCompletionResponse{
			ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []OpenAIChoice{{
				Index: 0,
				Message: OpenAIMessage{
					Role:    "assistant",
					Content: answer,
				},
				FinishReason: "stop",
			}},
		})
		return
	}

	reader, writer := io.Pipe()
	c.SetStatusCode(consts.StatusOK)
	c.Response.Header.Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Response.Header.Set("Cache-Control", "no-cache")
	c.Response.Header.Set("Connection", "keep-alive")
	c.SetBodyStream(reader, -1)

	go func() {
		defer writer.Close()

		_, err := h.chatService.StreamChatCompletion(ctx, userID, req.Model, messages, func(delta string) error {
			chunk := OpenAIChatCompletionResponse{
				ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []OpenAIChoice{{
					Index: 0,
					Delta: OpenAIMessage{Content: delta},
				}},
			}
			return writeOpenAIStreamChunk(writer, chunk)
		})
		if err != nil {
			_ = writeOpenAIStreamError(writer, err)
			return
		}

		finalChunk := OpenAIChatCompletionResponse{
			ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []OpenAIChoice{{Index: 0, FinishReason: "stop"}},
		}
		_ = writeOpenAIStreamChunk(writer, finalChunk)
		_, _ = fmt.Fprint(writer, "data: [DONE]\n\n")
	}()
}

func writeChatError(c *app.RequestContext, err error) {
	if errors.Is(err, service.ErrTooManyRequests) {
		c.JSON(consts.StatusTooManyRequests, Error(ecode.CodeTooManyRequests, "too many llm requests"))
		return
	}
	if err.Error() == "session not found" {
		c.JSON(consts.StatusNotFound, Error(ecode.CodeNotFound, "session not found"))
		return
	}
	c.JSON(consts.StatusInternalServerError, Error(ecode.CodeInternal, err.Error()))
}

func writeSSE(w io.Writer, event string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if event != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}

	return nil
}

func writeOpenAIStreamChunk(w io.Writer, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	return err
}

func writeOpenAIStreamError(w io.Writer, err error) error {
	return writeOpenAIStreamChunk(w, map[string]interface{}{
		"error": map[string]string{"message": err.Error()},
	})
}
