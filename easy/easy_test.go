package easy_test

import (
	"context"
	"fmt"
	"sync"
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

func TestLogger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var got string
	ctx = supervisor.WithContext(ctx, supervisor.WithLogger(func(a ...interface{}) {
		if got == "" {
			got = fmt.Sprint(a...)
		}
	}))

	var wg sync.WaitGroup
	wg.Add(1)
	supervisor.Add(ctx, func(context.Context) { wg.Done() }, supervisor.Transient)
	wg.Wait()

	const expected = "function service 1 starting"
	if got != expected {
		t.Error("unexpected logged message found. got:", got, "expected:", expected)
	}
}
