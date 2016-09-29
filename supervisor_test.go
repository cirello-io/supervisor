package supervisor

import (
	"fmt"
	"runtime/pprof"
	"testing"
	"time"

	"golang.org/x/net/context"
)

type Simpleservice int

func (s *Simpleservice) String() string {
	return fmt.Sprintf("simple service %d", int(*s))
}

func (s *Simpleservice) Serve(ctx context.Context) {
	var i int
	for {
		i++
		fmt.Println("service started:", *s, "iteration:", i)
		select {
		case <-ctx.Done():
			fmt.Println("context done:", ctx.Err(), *s)
			return
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func ExampleSupervisor() {
	var supervisor Supervisor

	svc := Simpleservice(1)
	supervisor.Add(&svc)

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.Serve(ctx)
}

func TestString(t *testing.T) {
	const expected = "test"
	var supervisor Supervisor
	supervisor.Name = expected

	if got := fmt.Sprintf("%s", &supervisor); got != expected {
		t.Errorf("error getting supervisor name: %s", got)
	}
}

func TestSimple(t *testing.T) {
	var supervisor Supervisor

	svc := Simpleservice(1)
	supervisor.Add(&svc)

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.Serve(ctx)

	countService(t, &supervisor)
}

func TestMultiple(t *testing.T) {
	var supervisor Supervisor

	svc1 := Simpleservice(2)
	supervisor.Add(&svc1)
	svc2 := Simpleservice(3)
	supervisor.Add(&svc2)

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.Serve(ctx)

	countService(t, &supervisor)
}

func TestCascaded(t *testing.T) {
	var supervisor Supervisor

	svc1 := Simpleservice(4)
	supervisor.Add(&svc1)
	svc2 := Simpleservice(5)
	supervisor.Add(&svc2)

	var childSupervisor Supervisor
	svc3 := Simpleservice(6)
	childSupervisor.Add(&svc3)
	svc4 := Simpleservice(7)
	childSupervisor.Add(&svc4)

	supervisor.Add(&childSupervisor)

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.Serve(ctx)

	countService(t, &supervisor)
}

type panicservice int

func (s *panicservice) Serve(ctx context.Context) {
	for {
		fmt.Println("panic service started:", *s)
		select {
		case <-ctx.Done():
			fmt.Println("panic service context:", ctx.Err(), *s)
			return
		default:
			time.Sleep(100 * time.Millisecond)
			panic("forcing panic")
		}
	}
}

func (s *panicservice) String() string {
	return fmt.Sprintf("panic service %v", *s)
}

func TestPanic(t *testing.T) {
	var supervisor Supervisor
	supervisor.Backoff = 500 * time.Millisecond
	svc1 := panicservice(1)
	supervisor.Add(&svc1)

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.Serve(ctx)

	countService(t, &supervisor)
}

type failingservice int

func (s *failingservice) Serve(ctx context.Context) {
	fmt.Println("failing service started:", *s, "times")
	select {
	case <-ctx.Done():
		fmt.Println("failing service context:", ctx.Err(), *s, "times")
		return
	default:
		time.Sleep(100 * time.Millisecond)
		*s++
		return
	}
}

func (s *failingservice) String() string {
	return fmt.Sprintf("failing service %v", *s)
}

func TestFailing(t *testing.T) {
	supervisor := Supervisor{
		Backoff: 1 * time.Second,
		Log: func(msg string) {
			t.Log("supervisor log:", msg)
		},
	}

	svc1 := failingservice(1)
	supervisor.Add(&svc1)

	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	supervisor.Serve(ctx)

	countService(t, &supervisor)
}

func TestAddServiceAfterServe(t *testing.T) {
	var supervisor Supervisor

	svc1 := Simpleservice(1)
	supervisor.Add(&svc1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	done := make(chan struct{})
	go func() {
		supervisor.Serve(ctx)
		done <- struct{}{}
	}()

	<-supervisor.startedServices
	svc2 := Simpleservice(2)
	supervisor.Add(&svc2)
	<-supervisor.startedServices

	cancel()
	<-ctx.Done()
	<-done

	countService(t, &supervisor)
}

func TestRemoveServiceAfterServe(t *testing.T) {
	var supervisor Supervisor

	svc1 := Simpleservice(1)
	supervisor.Add(&svc1)
	svc2 := Simpleservice(2)
	supervisor.Add(&svc2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	done := make(chan struct{})

	go func() {
		supervisor.Serve(ctx)
		done <- struct{}{}
	}()

	lbefore := getServiceCount(&supervisor)
	supervisor.Remove("unknown service")
	lafter := getServiceCount(&supervisor)

	if lbefore != lafter {
		t.Error("the removal of an unknown service shouldn't happen")
	}

	<-supervisor.startedServices
	supervisor.Remove(svc1.String())
	fmt.Println("removed service")

	lremoved := getServiceCount(&supervisor)
	if lbefore != lremoved {
		t.Error("the removal of a service should have affected the supervisor:", lbefore, lremoved)
	}

	cancel()
	<-ctx.Done()
	<-done

	countService(t, &supervisor)
}

func countService(t *testing.T, supervisor *Supervisor) {
	if r := supervisor.running; r != 0 {
		t.Errorf("not all services were stopped. possibly a bug: %d services left", r)
	}
}

func getServiceCount(s *Supervisor) int {
	s.servicesMu.Lock()
	l := len(s.services)
	s.servicesMu.Unlock()
	return l
}

func TestServices(t *testing.T) {
	var supervisor Supervisor

	svc1 := Simpleservice(1)
	supervisor.Add(&svc1)
	svc2 := Simpleservice(2)
	supervisor.Add(&svc2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	done := make(chan struct{})

	go func() {
		supervisor.Serve(ctx)
		done <- struct{}{}
	}()

	<-supervisor.startedServices
	svcs := supervisor.Services()
	fmt.Println(svcs)
	for _, svcname := range []string{svc1.String(), svc2.String()} {
		if _, ok := svcs[svcname]; !ok {
			t.Errorf("expected service not found: %s", svcname)
		}
	}

	cancel()
	<-ctx.Done()
	<-done

}

type restartableservice struct {
	id        int
	restarted chan struct{}
}

func (s *restartableservice) Serve(ctx context.Context) {
	fmt.Println("restartable service started:", *s)
	var i int
	for {
		i++
		select {
		case <-ctx.Done():
			fmt.Println("restartableservice service context:", ctx.Err(), s.id, i)
			return
		default:
			fmt.Println("restartableservice service loop:", s.id, i)
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

func TestManualCancelation(t *testing.T) {
	var supervisor Supervisor

	svc1 := Simpleservice(1)
	supervisor.Add(&svc1)
	svc2 := restartableservice{2, make(chan struct{})}
	supervisor.Add(&svc2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	done := make(chan struct{})

	go func() {
		supervisor.Serve(ctx)
		done <- struct{}{}
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
	<-done

	countService(t, &supervisor)
}

func TestServiceList(t *testing.T) {
	var supervisor Supervisor

	svc1 := Simpleservice(1)
	supervisor.Add(&svc1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	done := make(chan struct{})

	go func() {
		supervisor.Serve(ctx)
		done <- struct{}{}
	}()

	<-supervisor.startedServices

	svcs := supervisor.Services()
	if svc, ok := svcs[svc1.String()]; !ok || &svc1 != svc.(*Simpleservice) {
		t.Errorf("could not find service when listing them. %s missing", svc1.String())
	}

	cancel()
	<-ctx.Done()
	<-done

	countService(t, &supervisor)
}

type waitservice struct {
	count int
}

func (s *waitservice) Serve(ctx context.Context) {
	s.count++
	<-ctx.Done()
}

func (s *waitservice) String() string {
	return fmt.Sprintf("wait service %v", s.count)
}

func TestDoubleStart(t *testing.T) {
	var supervisor Supervisor

	var svc1 waitservice
	supervisor.Add(&svc1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	done := make(chan struct{})

	go func() {
		supervisor.Serve(ctx)
		done <- struct{}{}
	}()
	go func() {
		supervisor.Serve(ctx)
		done <- struct{}{}
	}()

	<-supervisor.startedServices

	if svc1.count != 1 {
		t.Error("wait service should have been started only once:", svc1.count)
	}

	cancel()
	<-ctx.Done()
	<-done
	<-done

	countService(t, &supervisor)
}

func TestRestart(t *testing.T) {

	var supervisor Supervisor

	var svc1 waitservice
	supervisor.Add(&svc1)

	for i := 0; i < 2; i++ {
		done := make(chan struct{})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		go func() {
			supervisor.Serve(ctx)
			done <- struct{}{}
		}()
		<-supervisor.startedServices

		cancel()
		<-ctx.Done()
		<-done
	}

	if svc1.count != 2 {
		t.Error("wait service should have been started twice:", svc1.count)
	}

	countService(t, &supervisor)
}

func TestProfile(t *testing.T) {
	for _, p := range pprof.Profiles() {
		t.Logf("%v %v", p.Name(), p.Count())
	}
}
