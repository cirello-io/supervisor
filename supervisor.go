package supervisor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

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

	// MaxRestarts is the number of maximum restarts given MaxTime. If more
	// than MaxRestarts occur in the last MaxTime, then the supervisor
	// stops all services and halts. Set this to AlwaysRestart to prevent
	// supervisor halt.
	MaxRestarts int

	// MaxTime is the time period on which the internal restart count will
	// be reset.
	MaxTime time.Duration

	// Log is a replaceable function used for overall logging.
	// Default: log.Printf.
	Log func(interface{})

	// indicates that supervisor is ready for use.
	prepared sync.Once

	// signals that a new service has just been added, so the started
	// supervisor picks it up.
	added chan struct{}

	// indicates that supervisor has running services.
	running         sync.Mutex
	runningServices sync.WaitGroup

	mu           sync.Mutex
	services     map[string]service            // added services
	cancelations map[string]context.CancelFunc // each service cancelation
	terminations map[string]context.CancelFunc // each service termination call
	lastRestart  time.Time
	restarts     int
}

func (s *Supervisor) prepare() {
	s.prepared.Do(s.reset)
}

func (s *Supervisor) reset() {
	s.mu.Lock()
	if s.Name == "" {
		s.Name = "supervisor"
	}
	if s.MaxRestarts == 0 {
		s.MaxRestarts = 5
	}
	if s.MaxTime == 0 {
		s.MaxTime = 15 * time.Second
	}
	if s.Log == nil {
		s.Log = func(msg interface{}) {
			log.Printf("%s: %v", s.Name, msg)
		}
	}

	s.added = make(chan struct{})
	s.cancelations = make(map[string]context.CancelFunc)
	s.services = make(map[string]service)
	s.terminations = make(map[string]context.CancelFunc)
	s.mu.Unlock()
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
	restartCtx, cancel := context.WithCancel(ctx)
	processFailure := func() {
		restart := s.shouldRestart()
		if !restart {
			cancel()
		}
	}
	serve(s, restartCtx, processFailure)
}

func (s *Supervisor) shouldRestart() bool {
	if s.MaxRestarts == AlwaysRestart {
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.lastRestart) > s.MaxTime {
		s.restarts = 0
	}
	s.lastRestart = time.Now()
	s.restarts++
	return s.restarts < s.MaxRestarts
}

func serve(s *Supervisor, ctx context.Context, processFailure processFailure) {
	s.running.Lock()
	defer s.running.Unlock()

	startServices(s, ctx, processFailure)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for {
			select {
			case <-s.added:
				startServices(s, ctx, processFailure)

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

func startServices(s *Supervisor, supervisorCtx context.Context, processFailure processFailure) {
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

		go func(name string, svc service) {
			s.runningServices.Add(1)
			defer s.runningServices.Done()
			wg.Done()
			retry := true
			for retry {
				retry = svc.svctype == Permanent
				s.Log(fmt.Sprintf("%s starting", name))
				func() {
					defer func() {
						if r := recover(); r != nil {
							s.Log(fmt.Sprintf("%s panic: %v", name, r))
							retry = svc.svctype == Permanent || svc.svctype == Transient
						}
					}()
					ctx, cancel := context.WithCancel(terminateCtx)
					s.mu.Lock()
					s.cancelations[name] = cancel
					s.mu.Unlock()
					svc.svc.Serve(ctx)
				}()
				if retry {
					processFailure()
				}
				select {
				case <-terminateCtx.Done():
					s.Log(fmt.Sprintf("%s restart aborted (terminated)", name))
					return
				case <-supervisorCtx.Done():
					s.Log(fmt.Sprintf("%s restart aborted (supervisor halted)", name))
					return
				default:
				}
				switch svc.svctype {
				case Temporary:
					s.Log(fmt.Sprintf("%s exited (temporary)", name))
					return
				case Transient:
					s.Log(fmt.Sprintf("%s exited (transient)", name))
				default:
					s.Log(fmt.Sprintf("%s exited (permanent)", name))
				}
			}
		}(name, svc)
	}
	wg.Wait()
}

// Group is a superset of Supervisor datastructure responsible for offering a
// supervisor tree whose all services are restarted whenever one of them fail or
// is restarted. It assumes that all services rely on each other. It does not
// guarantee any start other, but it does guarantee all services will be
// restarted. It implements Service, therefore it can be nested if necessary
// either with other Group or Supervisor. When passing the Group around,
// remind to do it as reference (&supervisor).
type Group struct {
	*Supervisor
}

// Serve starts the Group tree. It can be started only once at a time. If
// stopped (canceled), it can be restarted. In case of concurrent calls, it will
// hang until the current call is completed.
func (g *Group) Serve(ctx context.Context) {
	if g.Supervisor == nil {
		panic("Supervisor missing for this Group.")
	}
	g.Supervisor.prepare()
	restartCtx, cancel := context.WithCancel(ctx)

	var (
		mu                sync.Mutex
		processingFailure bool
	)
	processFailure := func() {
		mu.Lock()
		if processingFailure {
			mu.Unlock()
			return
		}
		processingFailure = true
		mu.Unlock()

		if !g.shouldRestart() {
			cancel()
			return
		}

		g.mu.Lock()
		g.Log("halting all services after failure")
		for _, c := range g.terminations {
			c()
		}

		g.cancelations = make(map[string]context.CancelFunc)
		g.mu.Unlock()

		go func() {
			g.Log("waiting for all services termination")
			g.runningServices.Wait()
			g.Log("waiting for all services termination - completed")

			mu.Lock()
			processingFailure = false
			mu.Unlock()

			g.Log("triggering group restart")
			g.added <- struct{}{}
		}()
	}
	serve(g.Supervisor, restartCtx, processFailure)
}

func contextWithCancel() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

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
