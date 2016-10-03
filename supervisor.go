package supervisor

import "fmt"

// AlwaysRestart adjusts the supervisor to never halt in face of failures.
const AlwaysRestart = -1

type processFailure func()

func (s *Supervisor) String() string {
	return s.Name
}

// Add inserts into the Supervisor tree a new service. If the Supervisor is
// already started, it will start it automatically. If the same service is added
// more than once, it will reset its backoff mechanism and force a service
// restart.
func (s *Supervisor) Add(service Service) {
	s.prepare()

	name := fmt.Sprintf("%s", service)

	s.mu.Lock()
	s.services[name] = service
	s.mu.Unlock()

	select {
	case s.added <- struct{}{}:
	default:
	}
}

// Remove stops the service in the Supervisor tree and remove from it.
func (s *Supervisor) Remove(name string) {
	s.prepare()

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.services[name]; !ok {
		return
	}

	delete(s.services, name)

	if c, ok := s.terminations[name]; ok {
		delete(s.terminations, name)
		c()
	}

	if _, ok := s.cancelations[name]; ok {
		delete(s.cancelations, name)
	}
}

// Services return a list of services
func (s *Supervisor) Services() map[string]Service {
	svclist := make(map[string]Service)
	s.mu.Lock()
	for k, v := range s.services {
		svclist[k] = v
	}
	s.mu.Unlock()
	return svclist
}

func (s *Supervisor) cleanchan() {
	select {
	case <-s.added:
	default:
	}
}
