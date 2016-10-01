package supervisor

import "sync"

var (
	// DefaultSupervisor is the default Supervisor used in this package.
	DefaultSupervisor Supervisor

	running sync.Mutex
)

func init() {
	DefaultSupervisor.Name = "default supervisor"
}

// Remove stops the service in the DefaultSupervisor tree and remove from it.
func Remove(name string) {
	DefaultSupervisor.Remove(name)
}

// Services return a list of services of DefaultSupervisor.
func Services() map[string]Service {
	return DefaultSupervisor.Services()
}

// Serve starts the DefaultSupervisor tree. It can be started only once at a
// time. If stopped (canceled), it can be restarted. In case of concurrent
// calls, it will hang until the current call is completed. It can run only one
// per package-level. If you need many, use
// supervisor.Supervisor/supervisor.Group instead of supervisor.Serve{,Group}.
// After its conclusion, its internal state is reset.
func Serve() {
	running.Lock()
	resetDefaultContext()
	ServeContext(defaultContext)
	running.Unlock()
}

// ServeGroup starts the DefaultSupervisor tree within a Group. It can be
// started only once at a time. If stopped (canceled), it can be restarted.
// In case of concurrent calls, it will hang until the current call is
// completed.  It can run only one per package-level. If you need many, use
// supervisor.ServeContext/supervisor.ServeGroupContext instead of
// supervisor.Serve/supervisor.ServeGroup.  After its conclusion, its internal
// state is reset.
func ServeGroup() {
	running.Lock()
	resetDefaultContext()
	ServeGroupContext(defaultContext)
	running.Unlock()
}
