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
