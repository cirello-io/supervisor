package supervisor

import (
	"fmt"
	"math"
	"sync"
	"time"
)

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
	s.backoff[name] = &backoff{
		decay:     s.FailureDecay,
		threshold: s.FailureThreshold,
		backoff:   s.Backoff,
		log: func(msg interface{}) {
			s.Log(fmt.Sprintf("backoff %s: %v", name, msg))
		},
	}
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

type backoff struct {
	decay     float64
	threshold float64
	backoff   time.Duration
	log       func(str interface{})

	lastfail time.Time
	failures float64
}

func (b *backoff) wait() {
	if b.lastfail.IsZero() {
		b.lastfail = time.Now()
		b.failures = 1.0
		return
	}

	b.failures++
	intervals := time.Since(b.lastfail).Seconds() / b.decay
	b.failures = b.failures*math.Pow(.5, intervals) + 1

	if b.failures > b.threshold {
		b.log(b.backoff)
		time.Sleep(b.backoff)
	}
}

type failingservice struct {
	id, count int
}

type panicservice struct {
	id, count int
}

type restartableservice struct {
	id        int
	restarted chan struct{}
}

type simpleservice int

type waitservice struct {
	id    int
	mu    sync.Mutex
	count int
}
