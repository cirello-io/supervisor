/*
Package easy is an easier interface to use cirello.io/supervisor. Its lifecycle
is managed through context.Context. Stop a given supervisor by cancelling its
context.


	package main

	import supervisor "cirello.io/supervisor/easy"

	func main() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// use cancel() to stop the supervisor
		ctx = supervisor.WithContext(ctx)
		supervisor.Add(ctx, func(ctx context.Context) {
			// ...
		})
	}
*/
package easy

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"cirello.io/supervisor"
)

type ctxKey int

const supervisorName ctxKey = 0

var (
	// ErrNoSupervisorAttached means that the given context has not been
	// wrapped with WithContext, and thus this package cannot detect
	// which supervisore you are referring to.
	ErrNoSupervisorAttached = errors.New("no supervisor attached to context")

	mu          sync.Mutex
	supervisors map[string]*supervisor.Group // map of supevisor name to supervisor.Supervisor
)

func init() {
	supervisors = make(map[string]*supervisor.Group)
}

// Permanent services are always restarted.
func Permanent() supervisor.ServiceOption {
	return supervisor.Permanent
}

// Transient services are restarted only when panic.
func Transient() supervisor.ServiceOption {
	return supervisor.Transient
}

// Temporary services are never restarted.
func Temporary() supervisor.ServiceOption {
	return supervisor.Temporary
}

// Add inserts supervised function to the attached supervisor, it launches
// automatically. If the context is not correctly prepared, it returns an
// ErrNoSupervisorAttached error
func Add(ctx context.Context, f func(context.Context), opts ...supervisor.ServiceOption) (string, error) {
	name, ok := extractName(ctx)
	if !ok {
		return "", ErrNoSupervisorAttached
	}
	mu.Lock()
	svr, ok := supervisors[name]
	mu.Unlock()
	if !ok {
		panic("supervisor not found")
	}
	svcName := svr.AddFunc(f, opts...)
	return svcName, nil
}

// Remove stops and removes the given service from the attached supervisor. If
// the context is not correctly prepared, it returns an ErrNoSupervisorAttached
// error
func Remove(ctx context.Context, name string) error {
	name, ok := extractName(ctx)
	if !ok {
		return ErrNoSupervisorAttached
	}
	mu.Lock()
	svr, ok := supervisors[name]
	mu.Unlock()
	if !ok {
		panic("supervisor not found")
	}
	svr.Remove(name)
	return nil
}

// WithContext takes a context and prepare it to be used by easy supervisor
// package. Internally, it creates a supervisor in group mode. In this mode,
// every time a service dies, the whole supervisor is restarted.
func WithContext(ctx context.Context) context.Context {
	chosenName := fmt.Sprintf("supervisor-%d", rand.Uint64())

	wrapped := context.WithValue(ctx, supervisorName, chosenName)
	svr := &supervisor.Group{
		Supervisor: &supervisor.Supervisor{
			MaxRestarts: supervisor.AlwaysRestart,
			Log:         func(interface{}) {},
		},
	}
	mu.Lock()
	supervisors[chosenName] = svr
	mu.Unlock()
	go svr.Serve(wrapped)

	return wrapped
}

func extractName(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(supervisorName).(string)
	return name, ok
}
