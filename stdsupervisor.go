package supervisor

var (
	// DefaultSupervisor is the default Supervisor used in this package.
	DefaultSupervisor Supervisor
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
// calls, it will hang until the current call is completed.
func Serve() {
	ServeContext(defaultContext)
}
