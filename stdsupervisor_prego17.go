// +build !go1.7

package supervisor

import "golang.org/x/net/context"

var (
	defaultContext context.Context

	// Cancel signal for the internal context
	Cancel context.CancelFunc
)

func init() {
	resetDefaultContext()
}

func resetDefaultContext() {
	defaultContext, Cancel = context.WithCancel(context.Background())
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
	DefaultSupervisor.Serve(ctx)
	DefaultSupervisor.reset()
}

// ServeGroupContext starts the DefaultSupervisor tree with a custom
// context.Context. It can be started only once at a time. If stopped
// (canceled), it can be restarted. In case of concurrent calls, it will hang
// until the current call is completed. After its conclusion, its internal
// state is reset.
func ServeGroupContext(ctx context.Context) {
	var group Group
	group.Supervisor = &DefaultSupervisor
	group.Serve(ctx)
	DefaultSupervisor.reset()
}
