package tool

import "context"

type Result struct {
	Name   string
	Output string
}

type Tool interface {
	Name() string
	Description() string
	ShouldUse(input string) bool
	Call(ctx context.Context, input string) (string, error)
}

type Registry struct {
	tools []Tool
}

func NewDefaultRegistry() *Registry {
	return &Registry{
		tools: []Tool{
			CalculatorTool{},
			TimeTool{},
		},
	}
}

func (r *Registry) RunMatched(ctx context.Context, input string) []Result {
	if r == nil {
		return nil
	}

	results := make([]Result, 0)
	for _, t := range r.tools {
		if !t.ShouldUse(input) {
			continue
		}

		output, err := t.Call(ctx, input)
		if err != nil {
			output = "tool error: " + err.Error()
		}

		results = append(results, Result{
			Name:   t.Name(),
			Output: output,
		})
	}

	return results
}
