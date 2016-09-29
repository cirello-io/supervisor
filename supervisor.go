/*
Package supervisor provides supervisor trees for Go applications.

This package is a clean reimplementation of github.com/thejerf/suture, aiming
to be more Go idiomatic, thus less Erlang-like.

It is built on top of context package, with all of its advantages, namely the
possibility trickle down context-related values and cancellation signals.

TheJerf's blog post about Suture is a very good and helpful read to understand
how this package has been implemented.

http://www.jerf.org/iri/post/2930
*/
package supervisor // import "cirello.io/supervisor"

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"golang.org/x/net/context"
)

// Service is the public interface expected by a Supervisor.
//
// This will be internally named after the result of fmt.Stringer, if available.
// Otherwise it will going to use an internal representation for the service
// name.
type Service interface {
	// Serve is called by a Supervisor to start the service. It expects the
	// service to honor the passed context and its lifetime. Observe
	// <-ctx.Done() and ctx.Err(). If the service is stopped by anything
	// but the Supervisor, it will get started again. Be careful with shared
	// state among restarts.
	Serve(ctx context.Context)
}

// Supervisor is the basic datastructure responsible for offering a supervisor
// tree. It implements Service, therefore it can be nested if necessary. When
// passing the Supervisor around, remind to do it as reference (&supervisor).
type Supervisor struct {

	// Name for this supervisor tree, used for logging.
	Name string

	// FailureDecay is the timespan on which the current failure count will
	// be halved.
	FailureDecay float64

	// FailureThreshold is the maximum accepted number of failures, after
	// decay adjustment, that shall trigger the back-off wait.
	FailureThreshold float64

	// Backoff is the wait duration when hit threshold.
	Backoff time.Duration

	// Log is a replaceable function used for overall logging
	Log func(string)

	ready     sync.Once
	startOnce sync.Once

	addedService    chan struct{}
	startedServices chan struct{}
	stoppedService  chan struct{}

	servicesMu sync.Mutex
	services   map[string]Service

	cancellationsMu sync.Mutex
	cancellations   map[string]context.CancelFunc

	backoffMu sync.Mutex
	backoff   map[string]*backoff
}

func (s *Supervisor) String() string {
	return s.Name
}

func (s *Supervisor) prepare() {
	s.ready.Do(func() {
		if s.Name == "" {
			s.Name = "supervisor"
		}
		s.addedService = make(chan struct{}, 1)
		s.backoff = make(map[string]*backoff)
		s.cancellations = make(map[string]context.CancelFunc)
		s.services = make(map[string]Service)
		s.startedServices = make(chan struct{})
		s.stoppedService = make(chan struct{}, 1)

		if s.Log == nil {
			s.Log = func(str string) {
				log.Println(s.Name, ":", str)
			}
		}
		if s.FailureDecay == 0 {
			s.FailureDecay = 30
		}
		if s.FailureThreshold == 0 {
			s.FailureThreshold = 5
		}
		if s.Backoff == 0 {
			s.Backoff = 15 * time.Second
		}
	})
}

// Add inserts into the Supervisor tree a new service. If the Supervisor is
// already started, it will start it automatically.
func (s *Supervisor) Add(service Service) {
	s.prepare()

	name := fmt.Sprintf("%s", service)

	s.servicesMu.Lock()
	s.backoffMu.Lock()
	s.backoff[name] = &backoff{}
	s.services[name] = service
	s.backoffMu.Unlock()
	s.servicesMu.Unlock()

	select {
	case s.addedService <- struct{}{}:
	default:
	}
}

// Remove stops the service in the Supervisor tree and remove from it.
func (s *Supervisor) Remove(name string) {
	s.prepare()

	s.servicesMu.Lock()
	defer s.servicesMu.Unlock()

	if _, ok := s.services[name]; !ok {
		return
	}

	s.cancellationsMu.Lock()
	defer s.cancellationsMu.Unlock()
	if c, ok := s.cancellations[name]; ok {
		delete(s.cancellations, name)
		c()
	}
}

// Serve starts the Supervisor tree. It can be started only once.
func (s *Supervisor) Serve(ctx context.Context) {
	s.prepare()
	s.startOnce.Do(func() {
		s.serve(ctx)
	})
}

// Services return a list of service names and their cancellation calls
func (s *Supervisor) Services() map[string]context.CancelFunc {
	svclist := make(map[string]context.CancelFunc)
	s.cancellationsMu.Lock()
	for k, v := range s.cancellations {
		svclist[k] = v
	}
	s.cancellationsMu.Unlock()
	return svclist
}

func (s *Supervisor) serve(ctx context.Context) {
	go func(ctx context.Context) {
		for range s.addedService {
			s.startServices(ctx)
			select {
			case s.startedServices <- struct{}{}:
			default:
			}
		}
	}(ctx)

	<-ctx.Done()
	s.cancellationsMu.Lock()
	s.cancellations = make(map[string]context.CancelFunc)
	s.cancellationsMu.Unlock()

	for range s.stoppedService {
		s.servicesMu.Lock()
		l := len(s.services)
		s.servicesMu.Unlock()
		if l == 0 {
			return
		}
	}
}

func (s *Supervisor) startServices(ctx context.Context) {
	s.servicesMu.Lock()
	defer s.servicesMu.Unlock()

	var wg sync.WaitGroup

	for name, svc := range s.services {
		s.cancellationsMu.Lock()
		_, ok := s.cancellations[name]
		s.cancellationsMu.Unlock()
		if ok {
			continue
		}

		wg.Add(1)

		go func(name string, svc Service) {
			wg.Done()
			for {
				retry := func() (retry bool) {
					select {
					case <-ctx.Done():
						return false
					default:
					}

					defer func() {
						if r := recover(); r != nil {
							s.Log(fmt.Sprint("trapped panic:", r))
							retry = true
						}
					}()

					c, cancel := context.WithCancel(ctx)
					s.cancellationsMu.Lock()
					s.cancellations[name] = cancel
					s.cancellationsMu.Unlock()
					svc.Serve(c)

					select {
					case <-ctx.Done():
						return false
					default:
						return true
					}
				}()
				if retry {
					s.Log(fmt.Sprintf("restarting %s", name))
					s.backoffMu.Lock()
					b := s.backoff[name]
					s.backoffMu.Unlock()
					b.wait(s.FailureDecay, s.FailureThreshold, s.Backoff)
					continue
				}

				s.servicesMu.Lock()
				delete(s.services, name)
				s.servicesMu.Unlock()
				s.stoppedService <- struct{}{}
				return
			}
		}(name, svc)
	}
	wg.Wait()
}

type backoff struct {
	lastfail time.Time
	failures float64
}

func (b *backoff) wait(failureDecay float64, threshold float64, backoffDur time.Duration) {
	if b.lastfail.IsZero() {
		b.lastfail = time.Now()
		b.failures = 1.0
	} else {
		b.failures++
		intervals := time.Since(b.lastfail).Seconds() / failureDecay
		b.failures = b.failures*math.Pow(.5, intervals) + 1
	}

	if b.failures > threshold {
		time.Sleep(backoffDur)
	}
}
