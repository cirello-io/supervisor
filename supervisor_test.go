package supervisor

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

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

	ctx, _ := contextWithTimeout(1 * time.Second)
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

	ctx, _ := contextWithTimeout(1 * time.Second)
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
		Backoff: 250 * time.Millisecond,
		Log: func(msg interface{}) {
			t.Log("supervisor log (cascaded with problems - child):", msg)
		},
	}
	svc3 := waitservice{id: 3}
	childSupervisor.Add(&svc3)
	svc4 := failingservice{id: 4}
	childSupervisor.Add(&svc4)

	supervisor.Add(&childSupervisor)

	ctx, _ := contextWithTimeout(5 * time.Second)
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

	ctx, _ := contextWithTimeout(1 * time.Second)
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

	ctx, _ := contextWithTimeout(3 * time.Second)
	supervisor.Serve(ctx)

	// should arrive here with no panic
	if svc1.count == 1 {
		t.Error("the failed service should have been started at least once.")
	}
}

func TestAddServiceAfterServe(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor
	svc1 := simpleservice(1)
	supervisor.Add(&svc1)

	ctx, cancel := contextWithTimeout(2 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()
	<-supervisor.started

	svc2 := simpleservice(2)
	supervisor.Add(&svc2)
	<-supervisor.started

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
	svc1 := simpleservice(1)
	supervisor.Add(&svc1)
	svc2 := simpleservice(2)
	supervisor.Add(&svc2)

	ctx, cancel := contextWithTimeout(10 * time.Second)
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

	<-supervisor.started

	supervisor.Remove(svc1.String())
	lremoved := getServiceCount(&supervisor)
	if lbefore == lremoved {
		t.Error("the removal of a service should have affected the supervisor:", lbefore, lremoved)
	}

	cancel()
	<-ctx.Done()
	wg.Wait()
}

func TestRemoveServiceAfterServeBug(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor
	svc1 := waitservice{id: 1}
	supervisor.Add(&svc1)

	ctx, _ := contextWithTimeout(2 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()
	<-supervisor.started

	supervisor.Remove(svc1.String())

	wg.Wait()

	if svc1.count > 1 {
		t.Errorf("the removal of a service should have terminated it. It was started %v times", svc1.count)
	}
}

func TestServices(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor
	svc1 := simpleservice(1)
	supervisor.Add(&svc1)
	svc2 := simpleservice(2)
	supervisor.Add(&svc2)

	ctx, cancel := contextWithTimeout(10 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
	}()
	<-supervisor.started

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
	svc1 := simpleservice(1)
	supervisor.Add(&svc1)
	svc2 := restartableservice{2, make(chan struct{})}
	supervisor.Add(&svc2)

	ctx, cancel := contextWithTimeout(10 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
	}()
	<-supervisor.started

	// Testing restart
	<-svc2.restarted
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
	svc1 := simpleservice(1)
	supervisor.Add(&svc1)

	ctx, cancel := contextWithTimeout(10 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
	}()
	<-supervisor.started

	svcs := supervisor.Services()
	if svc, ok := svcs[svc1.String()]; !ok || &svc1 != svc.(*simpleservice) {
		t.Errorf("could not find service when listing them. %s missing", svc1.String())
	}

	cancel()
	<-ctx.Done()
	wg.Done()
}

func TestRestart(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor
	var svc1 waitservice
	supervisor.Add(&svc1)

	for i := 1; i <= 20; i++ {
		ctx, _ := contextWithTimeout(time.Millisecond)
		supervisor.Serve(ctx)
		if svc1.count != i {
			t.Errorf("wait service should have been started %d. got: %d", i, svc1.count)
			break
		}
	}
}

func TestGroup(t *testing.T) {
	t.Parallel()

	supervisor := Group{
		Supervisor: &Supervisor{
			Log: func(msg interface{}) {
				t.Log("group log:", msg)
			},
		},
	}
	svc1 := holdingservice{id: 1}
	svc1.Add(1)
	supervisor.Add(&svc1)
	svc2 := holdingservice{id: 2}
	svc2.Add(1)
	supervisor.Add(&svc2)

	ctx, cancel := contextWithTimeout(3 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		// should arrive here with no panic
		if !(svc1.count == svc2.count && svc1.count == 2) {
			t.Errorf("affected service of a group should affect all. svc1.count: %d svc2.count: %d (both should be 2)", svc1.count, svc2.count)
		}
		wg.Done()
	}()
	svc1.Wait()
	svc2.Wait()

	svc1.Add(1)
	svc2.Add(1)

	cs := supervisor.Cancelations()
	cs[svc1.String()]()

	svc1.Wait()
	svc2.Wait()

	cancel()
	wg.Wait()
}

func TestPristineGroupServe(t *testing.T) {
	t.Parallel()

	var supervisor Group
	ctx, cancel := contextWithTimeout(3 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)

		if supervisor.Supervisor == nil {
			t.Errorf("internal supervisor should not be nil after first Serve.")
		}
		wg.Done()
	}()

	cancel()
	wg.Wait()
}

func (s *failingservice) String() string {
	return fmt.Sprintf("failing service %v", s.id)
}

func (s *panicservice) String() string {
	return fmt.Sprintf("panic service %v", s.id)
}

func (s *restartableservice) String() string {
	return fmt.Sprintf("restartable service %v", *s)
}

func (s *simpleservice) String() string {
	return fmt.Sprintf("simple service %d", int(*s))
}

func (s *waitservice) String() string {
	return fmt.Sprintf("wait service %v", s.id)
}

func (s *holdingservice) String() string {
	return fmt.Sprintf("holding service %v", s.id)
}

func getServiceCount(s *Supervisor) int {
	s.mu.Lock()
	l := len(s.services)
	s.mu.Unlock()
	return l
}
