package supervisor

import "fmt"

// AlwaysRestart adjusts the supervisor to never halt in face of failures.
const AlwaysRestart = -1

// ServiceType defines the restart strategy for a service.
type ServiceType int

const (
	// Permanent services are always restarted
	Permanent ServiceType = iota
	// Transient services are restarted only when panic.
	Transient
	// Temporary services are never restarted.
	Temporary
)

type service struct {
	svc     Service
	svctype ServiceType
}

type processFailure func()

func (s *Supervisor) String() string {
	return s.Name
}

// Add inserts into the Supervisor tree a new permanent service. If the
// Supervisor is already started, it will start it automatically. If the same
// service is added more than once, it will reset its backoff mechanism and
// force a service restart.
func (s *Supervisor) Add(service Service) {
	s.AddService(service, Permanent)
}

// AddService inserts into the Supervisor tree a new service of ServiceType. If
// the Supervisor is already started, it will start it automatically. If the
// same service is added more than once, it will reset its backoff mechanism and
// force a service restart.
func (s *Supervisor) AddService(svc Service, svctype ServiceType) {
	s.prepare()

	name := fmt.Sprintf("%s", svc)
	s.mu.Lock()
	s.services[name] = service{
		svc:     svc,
		svctype: svctype,
	}
	s.mu.Unlock()

	go func() {
		s.added <- struct{}{}
	}()
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
		svclist[k] = v.svc
	}
	s.mu.Unlock()
	return svclist
}
