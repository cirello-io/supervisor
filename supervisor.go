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
	// be halved. Default: 30s (represented in seconds).
	FailureDecay float64

	// FailureThreshold is the maximum accepted number of failures, after
	// decay adjustment, that shall trigger the back-off wait.
	// Default: 5 failures.
	FailureThreshold float64

	// Backoff is the wait duration when hit threshold. Default: 15s
	Backoff time.Duration

	// Log is a replaceable function used for overall logging.
	// Default: log.Printf.
	Log func(interface{})

	// indicates that supervisor is ready for use.
	prepared sync.Once

	// signals that a new service has just been added, so the started
	// supervisor picks it up.
	added chan struct{}

	// signals that confirm that at least one batch of added services has
	// been started. Used mainly for tests.
	started chan struct{}

	// indicates that supervisor is running, or has running services.
	running         sync.Mutex
	runningServices sync.WaitGroup

	mu           sync.Mutex
	services     map[string]Service            // added services
	cancelations map[string]context.CancelFunc // each service cancelation
	terminations map[string]context.CancelFunc // each service termination call
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
		if s.FailureDecay == 0 {
			s.FailureDecay = 30
		}
		if s.FailureThreshold == 0 {
			s.FailureThreshold = 5
		}
		if s.Backoff == 0 {
			s.Backoff = 15 * time.Second
		}
		if s.Log == nil {
			s.Log = func(msg interface{}) {
				log.Printf("%s: %v", s.Name, msg)
			}
		}

		s.added = make(chan struct{}, 1)
		s.backoff = make(map[string]*backoff)
		s.cancelations = make(map[string]context.CancelFunc)
		s.services = make(map[string]Service)
		s.started = make(chan struct{}, 1)
		s.terminations = make(map[string]context.CancelFunc)
	})
}

// Add inserts into the Supervisor tree a new service. If the Supervisor is
// already started, it will start it automatically.
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

// Cancelations return a list of services names and their cancellation calls.
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
}

func (s *Supervisor) startServices(supervisorCtx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var wg sync.WaitGroup
	for name, svc := range s.services {
		if _, ok := s.cancelations[name]; ok {
			continue
		}

		wg.Add(1)

		terminateCtx, terminate := context.WithCancel(supervisorCtx)
		s.cancelations[name] = terminate
		s.terminations[name] = terminate

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

					ctx, cancel := context.WithCancel(terminateCtx)
					s.mu.Lock()
					s.cancelations[name] = cancel
					s.mu.Unlock()
					svc.Serve(ctx)

					select {
					case <-terminateCtx.Done():
						return false
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
				b.wait()
			}
			s.runningServices.Done()
		}(name, svc)
	}
	wg.Wait()

	select {
	case s.started <- struct{}{}:
	default:
	}
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
