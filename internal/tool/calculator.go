package tool

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type CalculatorTool struct{}

func (CalculatorTool) Name() string { return "calculator" }

func (CalculatorTool) Description() string { return "calculate simple arithmetic expressions" }

func (CalculatorTool) ShouldUse(input string) bool {
	text := strings.ToLower(input)
	if strings.Contains(text, "计算") || strings.Contains(text, "calculate") || strings.Contains(text, "calculator") {
		return true
	}

	hasDigit := false
	hasOp := false
	for _, r := range text {
		if unicode.IsDigit(r) {
			hasDigit = true
		}
		if strings.ContainsRune("+-*/×÷", r) {
			hasOp = true
		}
	}
	return hasDigit && hasOp
}

func (CalculatorTool) Call(ctx context.Context, input string) (string, error) {
	expr := extractExpression(input)
	if expr == "" {
		return "", errors.New("no arithmetic expression found")
	}

	p := newExpressionParser(expr)
	value, err := p.parse()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s = %g", expr, value), nil
}

func extractExpression(input string) string {
	input = strings.ReplaceAll(input, "×", "*")
	input = strings.ReplaceAll(input, "÷", "/")

	best := ""
	var current strings.Builder
	flush := func() {
		candidate := strings.TrimSpace(current.String())
		if len(candidate) > len(best) && containsOperator(candidate) {
			best = candidate
		}
		current.Reset()
	}

	for _, r := range input {
		if unicode.IsDigit(r) || strings.ContainsRune("+-*/(). ", r) {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()

	return strings.TrimSpace(best)
}

func containsOperator(s string) bool {
	return strings.ContainsAny(s, "+-*/")
}

type expressionParser struct {
	s   string
	pos int
}

func newExpressionParser(s string) *expressionParser {
	return &expressionParser{s: s}
}

func (p *expressionParser) parse() (float64, error) {
	v, err := p.parseExpression()
	if err != nil {
		return 0, err
	}
	p.skipSpaces()
	if p.pos != len(p.s) {
		return 0, fmt.Errorf("unexpected token %q", p.s[p.pos:])
	}
	return v, nil
}

func (p *expressionParser) parseExpression() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}

	for {
		p.skipSpaces()
		if p.match('+') {
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			left += right
			continue
		}
		if p.match('-') {
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			left -= right
			continue
		}
		return left, nil
	}
}

func (p *expressionParser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}

	for {
		p.skipSpaces()
		if p.match('*') {
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			left *= right
			continue
		}
		if p.match('/') {
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, errors.New("division by zero")
			}
			left /= right
			continue
		}
		return left, nil
	}
}

func (p *expressionParser) parseFactor() (float64, error) {
	p.skipSpaces()

	if p.match('+') {
		return p.parseFactor()
	}
	if p.match('-') {
		v, err := p.parseFactor()
		return -v, err
	}
	if p.match('(') {
		v, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if !p.match(')') {
			return 0, errors.New("missing closing parenthesis")
		}
		return v, nil
	}

	return p.parseNumber()
}

func (p *expressionParser) parseNumber() (float64, error) {
	p.skipSpaces()
	start := p.pos
	seenDot := false
	for p.pos < len(p.s) {
		ch := p.s[p.pos]
		if ch == '.' && !seenDot {
			seenDot = true
			p.pos++
			continue
		}
		if ch < '0' || ch > '9' {
			break
		}
		p.pos++
	}

	if start == p.pos {
		return 0, errors.New("number expected")
	}

	return strconv.ParseFloat(p.s[start:p.pos], 64)
}

func (p *expressionParser) skipSpaces() {
	for p.pos < len(p.s) && p.s[p.pos] == ' ' {
		p.pos++
	}
}

func (p *expressionParser) match(ch byte) bool {
	if p.pos < len(p.s) && p.s[p.pos] == ch {
		p.pos++
		return true
	}
	return false
}
