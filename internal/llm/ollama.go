package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go-agent-chat-server/internal/model"
)

type OllamaClient struct {
	baseURL      string
	defaultModel string
	httpClient   *http.Client
}

func NewOllamaClient(baseURL string, defaultModel string) *OllamaClient {
	return &OllamaClient{
		baseURL:      strings.TrimRight(baseURL, "/"),
		defaultModel: defaultModel,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type OllamaChatResponse struct {
	Model   string        `json:"model"`
	Message OllamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

func (c *OllamaClient) Chat(ctx context.Context, modelName string, messages []model.Message) (string, error) {
	if modelName == "" {
		modelName = c.defaultModel
	}

	ollamaMessages := make([]OllamaMessage, 0, len(messages))
	for _, m := range messages {
		ollamaMessages = append(ollamaMessages, OllamaMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	reqBody := OllamaChatRequest{
		Model:    modelName,
		Messages: ollamaMessages,
		Stream:   false,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal ollama request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/chat",
		bytes.NewReader(data),
	)
	if err != nil {
		return "", fmt.Errorf("create ollama request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call ollama failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read ollama response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama status=%d body=%s", resp.StatusCode, string(body))
	}

	var result OllamaChatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("unmarshal ollama response failed: %w, body=%s", err, string(body))
	}

	return result.Message.Content, nil
}

func (c *OllamaClient) StreamChat(
	ctx context.Context,
	modelName string,
	messages []model.Message,
	onDelta func(delta string) error,
) (string, error) {
	if modelName == "" {
		modelName = c.defaultModel
	}

	ollamaMessages := make([]OllamaMessage, 0, len(messages))
	for _, m := range messages {
		ollamaMessages = append(ollamaMessages, OllamaMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	reqBody := OllamaChatRequest{
		Model:    modelName,
		Messages: ollamaMessages,
		Stream:   true,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal ollama stream request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/chat",
		bytes.NewReader(data),
	)
	if err != nil {
		return "", fmt.Errorf("create ollama stream request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call ollama stream failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama stream status=%d body=%s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var fullAnswer strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var item OllamaChatResponse
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return fullAnswer.String(), fmt.Errorf("unmarshal ollama stream chunk failed: %w, line=%s", err, line)
		}

		delta := item.Message.Content
		if delta != "" {
			fullAnswer.WriteString(delta)

			if onDelta != nil {
				if err := onDelta(delta); err != nil {
					return fullAnswer.String(), err
				}
			}
		}

		if item.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return fullAnswer.String(), fmt.Errorf("scan ollama stream failed: %w", err)
	}

	return fullAnswer.String(), nil
}
