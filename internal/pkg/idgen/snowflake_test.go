package idgen

import "testing"

func TestSnowflakeNextIDIsIncreasing(t *testing.T) {
	gen := NewSnowflake(1)
	first := gen.NextID()
	second := gen.NextID()
	if second <= first {
		t.Fatalf("expected second id > first id, got first=%d second=%d", first, second)
	}
}

func TestNewIDNotEmpty(t *testing.T) {
	id := NewID()
	if id == "" {
		t.Fatal("expected non-empty id")
	}
}
