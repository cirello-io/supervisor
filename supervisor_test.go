package supervisor

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"
)

func ExampleSupervisor() {
	var supervisor Supervisor

	svc := Simpleservice(1)
	supervisor.Add(&svc)

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.Serve(ctx)
}

func TestString(t *testing.T) {
	t.Parallel()

	const expected = "test"
	var supervisor Supervisor
	supervisor.Name = expected

	if got := fmt.Sprintf("%s", &supervisor); got != expected {
		t.Errorf("error getting supervisor name: %s", got)
	}
}

func TestCascaded(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor

	svc1 := waitservice{id: 1}
	supervisor.Add(&svc1)
	svc2 := waitservice{id: 2}
	supervisor.Add(&svc2)

	var childSupervisor Supervisor
	svc3 := waitservice{id: 3}
	childSupervisor.Add(&svc3)
	svc4 := waitservice{id: 4}
	childSupervisor.Add(&svc4)

	supervisor.Add(&childSupervisor)

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.Serve(ctx)

	if count := getServiceCount(&supervisor); count != 3 {
		t.Errorf("unexpected service count: %v", count)
	}

	switch {
	case svc1.count != 1, svc2.count != 1, svc3.count != 1, svc4.count != 1:
		t.Errorf("services should have been executed only once. %d %d %d %d",
			svc1.count, svc2.count, svc3.count, svc4.count)
	}
}

func TestLog(t *testing.T) {
	t.Parallel()

	supervisor := Supervisor{
		Backoff: 500 * time.Millisecond,
	}

	svc1 := panicservice{id: 1}
	supervisor.Add(&svc1)

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.Serve(ctx)
}

func TestCascadedWithProblems(t *testing.T) {
	t.Parallel()

	supervisor := Supervisor{
		Backoff: 1 * time.Second,
		Log: func(msg interface{}) {
			t.Log("supervisor log (cascaded with problems):", msg)
		},
	}
	svc1 := waitservice{id: 1}
	supervisor.Add(&svc1)
	svc2 := panicservice{id: 2}
	supervisor.Add(&svc2)

	childSupervisor := Supervisor{
		Backoff: 1 * time.Second,
		Log: func(msg interface{}) {
			t.Log("supervisor log (cascaded with problems - child):", msg)
		},
	}
	svc3 := waitservice{id: 3}
	childSupervisor.Add(&svc3)
	svc4 := failingservice{id: 4}
	childSupervisor.Add(&svc4)

	supervisor.Add(&childSupervisor)

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	supervisor.Serve(ctx)

	if count := getServiceCount(&supervisor); count != 3 {
		t.Errorf("unexpected service count: %v", count)
	}

	switch {
	case svc1.count != 1, svc3.count != 1:
		t.Errorf("services should have been executed only once. %d %d %d %d",
			svc1.count, svc2.count, svc3.count, svc4.count)
	case svc2.count <= 1, svc4.count <= 1:
		t.Errorf("services should have been executed at least once. %d %d %d %d",
			svc1.count, svc2.count, svc3.count, svc4.count)
	}
}

func TestPanic(t *testing.T) {
	t.Parallel()

	supervisor := Supervisor{
		Backoff: 500 * time.Millisecond,
		Log: func(msg interface{}) {
			t.Log("supervisor log (panic):", msg)
		},
	}
	svc1 := panicservice{id: 1}
	supervisor.Add(&svc1)

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.Serve(ctx)

	// should arrive here with no panic
	if svc1.count == 1 {
		t.Error("the failed service should have been started at least once.")
	}
}

func TestFailing(t *testing.T) {
	t.Parallel()

	supervisor := Supervisor{
		Backoff: 1 * time.Second,
		Log: func(msg interface{}) {
			t.Log("supervisor log (failing):", msg)
		},
	}

	svc1 := failingservice{id: 1}
	supervisor.Add(&svc1)

	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	supervisor.Serve(ctx)

	// should arrive here with no panic
	if svc1.count == 1 {
		t.Error("the failed service should have been started at least once.")
	}
}

func TestAddServiceAfterServe(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor

	svc1 := Simpleservice(1)
	supervisor.Add(&svc1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()

	<-supervisor.startedServices
	svc2 := Simpleservice(2)
	supervisor.Add(&svc2)
	<-supervisor.startedServices

	cancel()
	<-ctx.Done()
	wg.Wait()

	if count := getServiceCount(&supervisor); count != 2 {
		t.Errorf("unexpected service count: %v", count)
	}
}

func TestRemoveServiceAfterServe(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor

	svc1 := Simpleservice(1)
	supervisor.Add(&svc1)
	svc2 := Simpleservice(2)
	supervisor.Add(&svc2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()

	lbefore := getServiceCount(&supervisor)
	supervisor.Remove("unknown service")
	lafter := getServiceCount(&supervisor)

	if lbefore != lafter {
		t.Error("the removal of an unknown service shouldn't happen")
	}

	<-supervisor.startedServices
	supervisor.Remove(svc1.String())

	lremoved := getServiceCount(&supervisor)
	if lbefore != lremoved {
		t.Error("the removal of a service should have affected the supervisor:", lbefore, lremoved)
	}

	cancel()
	<-ctx.Done()
	wg.Wait()
}

func TestServices(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor

	svc1 := Simpleservice(1)
	supervisor.Add(&svc1)
	svc2 := Simpleservice(2)
	supervisor.Add(&svc2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
	}()

	<-supervisor.startedServices
	svcs := supervisor.Services()
	for _, svcname := range []string{svc1.String(), svc2.String()} {
		if _, ok := svcs[svcname]; !ok {
			t.Errorf("expected service not found: %s", svcname)
		}
	}

	cancel()
	<-ctx.Done()
	wg.Done()
}

func TestManualCancelation(t *testing.T) {
	t.Parallel()

	supervisor := Supervisor{
		Log: func(msg interface{}) {
			t.Log("supervisor log (restartable):", msg)
		},
	}

	svc1 := Simpleservice(1)
	supervisor.Add(&svc1)
	svc2 := restartableservice{2, make(chan struct{})}
	supervisor.Add(&svc2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
	}()

	<-supervisor.startedServices
	<-svc2.restarted

	// Testing restart
	svcs := supervisor.Cancelations()
	svcancel := svcs[svc2.String()]
	svcancel()
	<-svc2.restarted

	cancel()
	<-ctx.Done()
	wg.Done()

	// should arrive here with no panic
}

func TestServiceList(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor

	svc1 := Simpleservice(1)
	supervisor.Add(&svc1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
	}()

	<-supervisor.startedServices

	svcs := supervisor.Services()
	if svc, ok := svcs[svc1.String()]; !ok || &svc1 != svc.(*Simpleservice) {
		t.Errorf("could not find service when listing them. %s missing", svc1.String())
	}

	cancel()
	<-ctx.Done()
	wg.Done()
}

func TestDoubleStart(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor

	var svc1 waitservice
	supervisor.Add(&svc1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			c := context.WithValue(ctx, "supervisor", i)
			supervisor.Serve(c)

			svc1.mu.Lock()
			count := svc1.count
			supervisors := svc1.supervisors
			if count > 1 {
				t.Error("wait service should have been started once:", count, "supervisor IDs:", supervisors)
			}
			svc1.mu.Unlock()

			wg.Done()
		}(i)
	}
	<-supervisor.startedServices

	cancel()
	<-ctx.Done()
	wg.Wait()
}

func TestRestart(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor

	var svc1 waitservice
	supervisor.Add(&svc1)

	for i := 1; i <= 3; i++ {
		ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
		supervisor.Serve(ctx)
		if svc1.count != i {
			t.Errorf("wait service should have been started %d. got: %d", i, svc1.count)
		}
	}
}

type failingservice struct {
	id, count int
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

func (s *failingservice) String() string {
	return fmt.Sprintf("failing service %v", s.id)
}

type panicservice struct {
	id, count int
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

func (s *panicservice) String() string {
	return fmt.Sprintf("panic service %v", s.id)
}

type restartableservice struct {
	id        int
	restarted chan struct{}
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

func (s *restartableservice) String() string {
	return fmt.Sprintf("restartable service %v", *s)
}

type Simpleservice int

func (s *Simpleservice) String() string {
	return fmt.Sprintf("simple service %d", int(*s))
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

type waitservice struct {
	id          int
	mu          sync.Mutex
	count       int
	supervisors []int
}

func (s *waitservice) Serve(ctx context.Context) {
	s.mu.Lock()
	s.count++
	id := ctx.Value("supervisor")
	if id != nil {
		s.supervisors = append(s.supervisors, id.(int))
	}
	s.mu.Unlock()
	<-ctx.Done()
}

func (s *waitservice) String() string {
	return fmt.Sprintf("wait service %v", s.id)
}

func getServiceCount(s *Supervisor) int {
	s.servicesMu.Lock()
	l := len(s.services)
	s.servicesMu.Unlock()
	return l
}
