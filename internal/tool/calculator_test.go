package tool

import (
	"context"
	"strings"
	"testing"
)

func TestCalculatorTool(t *testing.T) {
	tool := CalculatorTool{}
	if !tool.ShouldUse("帮我计算 1 + 2 * 3") {
		t.Fatal("expected calculator to match arithmetic input")
	}

	result, err := tool.Call(context.Background(), "帮我计算 1 + 2 * 3")
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}
	if !strings.Contains(result, "= 7") {
		t.Fatalf("unexpected result: %s", result)
	}
}
