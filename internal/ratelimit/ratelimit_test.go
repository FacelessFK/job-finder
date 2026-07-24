package ratelimit

import (
	"context"
	"testing"
)

func TestZeroIntervalPasses(t *testing.T) {
	l := New(0)
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCanceledContext(t *testing.T) {
	l := New(0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Wait(ctx); err == nil {
		t.Fatal("expected context error")
	}
}
