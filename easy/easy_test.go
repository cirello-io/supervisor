package easy_test

import (
	"context"
	"testing"

	supervisor "cirello.io/supervisor/easy"
)

func TestInvalidContext(t *testing.T) {
	ctx := context.Background()
	_, err := supervisor.Add(ctx, func(context.Context) {})
	if err != supervisor.ErrNoSupervisorAttached {
		t.Errorf("ErrNoSupervisorAttached not found: %v", err)
	}

	if err := supervisor.Remove(ctx, "fake name"); err != supervisor.ErrNoSupervisorAttached {
		t.Errorf("ErrNoSupervisorAttached not found: %v", err)
	}
}
