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
	supervisors map[string]*supervisor.Group // map of supervisor name to supervisor.Supervisor
)

func init() {
	supervisors = make(map[string]*supervisor.Group)
}

var (
	// Permanent services are always restarted.
	Permanent = supervisor.Permanent

	// Transient services are restarted only when panic.
	Transient = supervisor.Transient

	// Temporary services are never restarted.
	Temporary = supervisor.Temporary
)

// Add inserts supervised function to the attached supervisor, it launches
// automatically. If the context is not correctly prepared, it returns an
// ErrNoSupervisorAttached error. By default, the restart policy is Permanent.
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
	opts = append([]supervisor.ServiceOption{Permanent}, opts...)
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
func WithContext(ctx context.Context, opts ...SupervisorOption) context.Context {
	chosenName := fmt.Sprintf("supervisor-%d", rand.Uint64())

	svr := &supervisor.Supervisor{
		Name:        chosenName,
		MaxRestarts: supervisor.AlwaysRestart,
		Log:         func(interface{}) {},
	}
	for _, opt := range opts {
		opt(svr)
	}
	group := &supervisor.Group{
		Supervisor: svr,
	}
	mu.Lock()
	supervisors[chosenName] = group
	mu.Unlock()

	wrapped := context.WithValue(ctx, supervisorName, chosenName)
	go group.Serve(wrapped)
	return wrapped
}

// SupervisorOption reconfigures the supervisor attached to the context.
type SupervisorOption func(*supervisor.Supervisor)

// WithLogger attaches a log function to the supervisor
func WithLogger(logger func(a ...interface{})) SupervisorOption {
	return func(s *supervisor.Supervisor) {
		s.Log = func(v interface{}) {
			logger(v)
		}
	}
}

func extractName(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(supervisorName).(string)
	return name, ok
}
