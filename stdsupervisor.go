package supervisor

import "golang.org/x/net/context"

var (
	// DefaultSupervisor is the default Supervisor used in this package.
	DefaultSupervisor Supervisor
	defaultContext    = context.Background()
)

func init() {
	DefaultSupervisor.Name = "default supervisor"
}

// Add inserts new service into the DefaultSupervisor. If the DefaultSupervisor
// is already started, it will start it automatically. If the same service is
// added more than once, it will reset its backoff mechanism and force a service
// restart.
func Add(service Service) {
	DefaultSupervisor.Add(service)
}

// Remove stops the service in the DefaultSupervisor tree and remove from it.
func Remove(name string) {
	DefaultSupervisor.Remove(name)
}

// Services return a list of services of DefaultSupervisor.
func Services() map[string]Service {
	return DefaultSupervisor.Services()
}

// Cancelations return a list of services names of DefaultSupervisor and their
// cancelation calls. These calls be used to force a service restart.
func Cancelations() map[string]context.CancelFunc {
	return DefaultSupervisor.Cancelations()
}

// Serve starts the DefaultSupervisor tree. It can be started only once at a
// time. If stopped (canceled), it can be restarted. In case of concurrent
// calls, it will hang until the current call is completed.
func Serve() {
	ServeContext(defaultContext)
}

// ServeContext starts the DefaultSupervisor tree with a custom context.Context.
// It can be started only once at a time. If stopped (canceled), it can be
// restarted. In case of concurrent calls, it will hang until the current call
// is completed.
func ServeContext(ctx context.Context) {
	DefaultSupervisor.Serve(ctx)
}
