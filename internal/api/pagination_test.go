package api

import "testing"

func TestParsePositiveInt(t *testing.T) {
	if got := parsePositiveInt("2", 1); got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}
	if got := parsePositiveInt("bad", 1); got != 1 {
		t.Fatalf("expected fallback 1, got %d", got)
	}
	if got := parsePositiveInt("0", 1); got != 1 {
		t.Fatalf("expected fallback 1, got %d", got)
	}
}
