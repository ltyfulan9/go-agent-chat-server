package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	appmodel "go-agent-chat-server/internal/model"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino/schema"
)

type EinoRunner struct {
	baseURL      string
	defaultModel string
}

func NewEinoRunner(baseURL string, defaultModel string) *EinoRunner {
	return &EinoRunner{
		baseURL:      strings.TrimRight(baseURL, "/"),
		defaultModel: defaultModel,
	}
}

func (r *EinoRunner) Chat(ctx context.Context, modelName string, messages []appmodel.Message) (string, error) {
	if modelName == "" {
		modelName = r.defaultModel
	}

	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: r.baseURL,
		Model:   modelName,
		Timeout: 60 * time.Second,
	})
	if err != nil {
		return "", fmt.Errorf("create eino ollama chat model failed: %w", err)
	}

	einoMessages := make([]*schema.Message, 0, len(messages))

	for _, m := range messages {
		role := schema.User

		switch m.Role {
		case "assistant":
			role = schema.Assistant
		case "system":
			role = schema.System
		default:
			role = schema.User
		}

		einoMessages = append(einoMessages, &schema.Message{
			Role:    role,
			Content: m.Content,
		})
	}

	resp, err := chatModel.Generate(ctx, einoMessages)
	if err != nil {
		return "", fmt.Errorf("eino generate failed: %w", err)
	}

	return resp.Content, nil
}
