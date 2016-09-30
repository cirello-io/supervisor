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
	Log func(interface{})

	// indicates that supervisor is ready for use.
	prepared sync.Once

	// signals that a new service has just been added, so the started
	// supervisor picks it up.
	added chan struct{}

	// signals that confirm that at least one batch of added services has
	// been started.
	// used mainly for tests
	started chan struct{}

	// indicates that supervisor is running, or has running services.
	running         sync.Mutex
	runningServices sync.WaitGroup

	mu           sync.Mutex
	services     map[string]Service            // added services
	cancelations map[string]context.CancelFunc // each service cancelation
	backoff      map[string]*backoff           // backoff calculator for failures
}

func (s *Supervisor) String() string {
	return s.Name
}

func (s *Supervisor) prepare() {
	s.prepared.Do(func() {
		if s.Name == "" {
			s.Name = "supervisor"
		}
		s.added = make(chan struct{}, 1)
		s.backoff = make(map[string]*backoff)
		s.cancelations = make(map[string]context.CancelFunc)
		s.services = make(map[string]Service)
		s.started = make(chan struct{}, 1)

		if s.Log == nil {
			s.Log = func(msg interface{}) {
				log.Printf("%s: %v", s.Name, msg)
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

	s.mu.Lock()
	s.backoff[name] = &backoff{}
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

	if c, ok := s.cancelations[name]; ok {
		delete(s.cancelations, name)
		c()
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

// Cancelations return a list of services names and their cancellation calls
func (s *Supervisor) Cancelations() map[string]context.CancelFunc {
	svclist := make(map[string]context.CancelFunc)
	s.mu.Lock()
	for k, v := range s.cancelations {
		svclist[k] = v
	}
	s.mu.Unlock()
	return svclist
}

// Serve starts the Supervisor tree. It can be started only once at a time. If
// stopped (canceled), it can be restarted. In case of concurrent calls, it will
// hang until the current call is completed.
func (s *Supervisor) Serve(ctx context.Context) {
	s.prepare()

	s.running.Lock()
	s.serve(ctx)
	s.running.Unlock()
}

func (s *Supervisor) serve(ctx context.Context) {
	select {
	case <-s.added:
	default:
	}
	s.startServices(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for {
			select {
			case <-s.added:
				s.startServices(ctx)

			case <-ctx.Done():
				wg.Done()
				return
			}
		}
	}()
	<-ctx.Done()

	wg.Wait()
	s.runningServices.Wait()

	s.mu.Lock()
	s.cancelations = make(map[string]context.CancelFunc)
	s.mu.Unlock()
	return
}

func (s *Supervisor) startServices(ctx context.Context) {
	s.startAllServices(ctx)
	select {
	case s.started <- struct{}{}:
	default:
	}
}

func (s *Supervisor) startAllServices(supervisorCtx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var wg sync.WaitGroup
	for name, svc := range s.services {
		if _, ok := s.cancelations[name]; ok {
			continue
		}

		wg.Add(1)

		// In the edge case that a service has been added, immediately
		// removed, but the service itself hasn't been started. This
		// intermediate context shall prevent a nil pointer in
		// Supervisor.Remove(), but also stops all the subsequent
		// service restarts. It might deserve a more elegant solution.
		intermediateCtx, cancel := context.WithCancel(supervisorCtx)
		s.cancelations[name] = cancel

		go func(name string, svc Service) {
			s.runningServices.Add(1)
			wg.Done()
			for {
				retry := func() (retry bool) {
					defer func() {
						if r := recover(); r != nil {
							s.Log(fmt.Sprintf("trapped panic: %v", r))
							retry = true
						}
					}()

					ctx, cancel := context.WithCancel(intermediateCtx)
					s.mu.Lock()
					s.cancelations[name] = cancel
					s.mu.Unlock()
					svc.Serve(ctx)

					// Only stops the service, if the supervisor wants to.
					// Otherwise, keep restarting.
					select {
					case <-supervisorCtx.Done():
						return false
					default:
						return true
					}
				}()
				if !retry {
					break
				}
				s.Log(fmt.Sprintf("restarting %s", name))
				s.mu.Lock()
				b := s.backoff[name]
				s.mu.Unlock()
				b.wait(s.FailureDecay, s.FailureThreshold, s.Backoff, func(msg interface{}) {
					s.Log(fmt.Sprintf("backoff %s: %v", name, msg))
				})
			}
			s.runningServices.Done()
		}(name, svc)
	}

	wg.Wait()
}

type backoff struct {
	lastfail time.Time
	failures float64
}

func (b *backoff) wait(failureDecay float64, threshold float64, backoffDur time.Duration, log func(str interface{})) {
	if b.lastfail.IsZero() {
		b.lastfail = time.Now()
		b.failures = 1.0
		return
	}

	b.failures++
	intervals := time.Since(b.lastfail).Seconds() / failureDecay
	b.failures = b.failures*math.Pow(.5, intervals) + 1

	if b.failures > threshold {
		log(backoffDur)
		time.Sleep(backoffDur)
	}
}
