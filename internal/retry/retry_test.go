package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoSucceedsAfterRetries(t *testing.T) {
	calls := 0
	err := Do(context.Background(), 3, time.Millisecond, func(error) bool { return true }, func() error {
		calls++
		if calls < 3 {
			return errors.New("temp")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDoStopsOnNonRetryable(t *testing.T) {
	calls := 0
	err := Do(context.Background(), 5, time.Millisecond, func(error) bool { return false }, func() error {
		calls++
		return errors.New("fatal")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}
