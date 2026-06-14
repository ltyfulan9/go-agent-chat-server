package tool

import (
	"context"
	"strings"
	"time"
)

type TimeTool struct{}

func (TimeTool) Name() string { return "time" }

func (TimeTool) Description() string { return "return current local time" }

func (TimeTool) ShouldUse(input string) bool {
	text := strings.ToLower(input)
	keywords := []string{"几点", "现在时间", "当前时间", "日期", "today", "time", "date"}
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func (TimeTool) Call(ctx context.Context, input string) (string, error) {
	return time.Now().Format("2006-01-02 15:04:05 MST"), nil
}
