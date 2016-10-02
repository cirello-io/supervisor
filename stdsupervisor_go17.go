// +build go1.7

package supervisor

import "context"

var (
	defaultContext = context.Background()
)

// SetDefaultContext allows to change the context used for supervisor.Serve()
// and supervisor.ServeGroup().
func SetDefaultContext(ctx context.Context) {
	running.Lock()
	defaultContext = ctx
	running.Unlock()
}

// Add inserts new service into the DefaultSupervisor. If the DefaultSupervisor
// is already started, it will start it automatically. If the same service is
// added more than once, it will reset its backoff mechanism and force a service
// restart.
func Add(service Service) {
	DefaultSupervisor.Add(service)
}

// Cancelations return a list of services names of DefaultSupervisor and their
// cancelation calls. These calls be used to force a service restart.
func Cancelations() map[string]context.CancelFunc {
	return DefaultSupervisor.Cancelations()
}

// ServeContext starts the DefaultSupervisor tree with a custom context.Context.
// It can be started only once at a time. If stopped (canceled), it can be
// restarted. In case of concurrent calls, it will hang until the current call
// is completed. After its conclusion, its internal state is reset.
func ServeContext(ctx context.Context) {
	running.Lock()
	DefaultSupervisor.Serve(ctx)
	DefaultSupervisor.reset()
	running.Unlock()
}

// ServeGroupContext starts the DefaultSupervisor tree with a custom
// context.Context. It can be started only once at a time. If stopped
// (canceled), it can be restarted. In case of concurrent calls, it will hang
// until the current call is completed. After its conclusion, its internal
// state is reset.
func ServeGroupContext(ctx context.Context) {
	running.Lock()
	var group Group
	group.Supervisor = &DefaultSupervisor
	group.Serve(ctx)
	DefaultSupervisor.reset()
	running.Unlock()
}

// Serve starts the DefaultSupervisor tree. It can be started only once at a
// time. If stopped (canceled), it can be restarted. In case of concurrent
// calls, it will hang until the current call is completed. It can run only one
// per package-level. If you need many, use
// supervisor.Supervisor/supervisor.Group instead of supervisor.Serve{,Group}.
// After its conclusion, its internal state is reset.
func Serve() {
	running.Lock()
	DefaultSupervisor.Serve(defaultContext)
	DefaultSupervisor.reset()
	defaultContext = context.Background()
	running.Unlock()
}

// ServeGroup starts the DefaultSupervisor tree within a Group. It can be
// started only once at a time. If stopped (canceled), it can be restarted.
// In case of concurrent calls, it will hang until the current call is
// completed.  It can run only one per package-level. If you need many, use
// supervisor.ServeContext/supervisor.ServeGroupContext instead of
// supervisor.Serve/supervisor.ServeGroup. After its conclusion, its internal
// state is reset.
func ServeGroup() {
	running.Lock()
	var group Group
	group.Supervisor = &DefaultSupervisor
	group.Serve(defaultContext)
	DefaultSupervisor.reset()
	defaultContext = context.Background()
	running.Unlock()
}
