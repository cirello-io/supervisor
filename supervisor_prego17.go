// +build !go1.7

package supervisor

import (
	"fmt"
	"log"
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

// Cancelations return a list of services names and their cancelation calls.
// These calls be used to force a service restart.
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

func contextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
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

func contextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

func (s *failingservice) Serve(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
		time.Sleep(100 * time.Millisecond)
		s.count++
		return
	}
}

func (s *panicservice) Serve(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(100 * time.Millisecond)
			s.count++
			panic("forcing panic")
		}
	}
}

func (s *restartableservice) Serve(ctx context.Context) {
	var i int
	for {
		i++
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(500 * time.Millisecond)
			select {
			case s.restarted <- struct{}{}:
			default:
			}
		}
	}
}

func (s *Simpleservice) Serve(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (s *waitservice) Serve(ctx context.Context) {
	s.mu.Lock()
	s.count++
	s.mu.Unlock()
	<-ctx.Done()
}
