package supervisor

import (
	"fmt"
	"io/ioutil"
	"log"
	"sync"
	"testing"
	"time"
)

func init() {
	log.SetOutput(ioutil.Discard)
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

func TestStringDefaultName(t *testing.T) {
	t.Parallel()

	const expected = "supervisor"
	var supervisor Supervisor
	supervisor.prepare()

	if got := fmt.Sprintf("%s", &supervisor); got != expected {
		t.Errorf("error getting supervisor name: %s", got)
	}
}

func TestCascaded(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor
	supervisor.Name = "TestCascaded root"
	svc1 := waitservice{id: 1}
	supervisor.Add(&svc1)
	svc2 := waitservice{id: 2}
	supervisor.Add(&svc2)

	var childSupervisor Supervisor
	childSupervisor.Name = "TestCascaded child"
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
		Name: "TestLog",
	}
	svc1 := panicservice{id: 1}
	supervisor.Add(&svc1)

	ctx, _ := contextWithTimeout(1 * time.Second)
	supervisor.Serve(ctx)
}

func TestCascadedWithProblems(t *testing.T) {
	t.Parallel()

	supervisor := Supervisor{
		Name: "TestCascadedWithProblems root",
		Log: func(msg interface{}) {
			t.Log("supervisor log (cascaded with problems):", msg)
		},
	}
	svc1 := waitservice{id: 1}
	supervisor.Add(&svc1)
	svc2 := panicservice{id: 2}
	supervisor.Add(&svc2)
	svcHolding := &holdingservice{id: 98}
	svcHolding.Add(1)
	supervisor.Add(svcHolding)

	childSupervisor := Supervisor{
		Name: "TestCascadedWithProblems child",
		Log: func(msg interface{}) {
			t.Log("supervisor log (cascaded with problems - child):", msg)
		},
	}
	svc3 := waitservice{id: 3}
	childSupervisor.Add(&svc3)
	svc4 := failingservice{id: 4}
	childSupervisor.Add(&svc4)
	svcHolding2 := &holdingservice{id: 99}
	svcHolding2.Add(1)
	childSupervisor.Add(svcHolding2)

	supervisor.Add(&childSupervisor)

	ctx, _ := contextWithTimeout(5 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()

	svcHolding.Wait()
	svcHolding2.Wait()
	wg.Wait()

	if count := getServiceCount(&supervisor); count != 4 {
		t.Errorf("unexpected service count: %v", count)
	}

	switch {
	case svc1.count != 1, svc3.count != 1:
		t.Errorf("services should have been executed only once. svc1.count:%d svc3.count: %d",
			svc1.count, svc3.count)
	case svc2.count <= 1, svc4.count <= 1:
		t.Errorf("services should have been executed at least once. svc2.count:%d svc4.count: %d",
			svc2.count, svc4.count)
	}
}

func TestPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	supervisor := Supervisor{
		Name: "TestPanic supervisor",
		Log: func(msg interface{}) {
			t.Log("supervisor log (panic):", msg)
		},
	}
	svc1 := panicservice{id: 1}
	supervisor.Add(&svc1)

	ctx, _ := contextWithTimeout(1 * time.Second)
	supervisor.Serve(ctx)

	if svc1.count == 1 {
		t.Error("the failed service should have been started at least once.")
	}
}

func TestRemovePanicService(t *testing.T) {
	t.Parallel()

	supervisor := Group{
		Supervisor: &Supervisor{
			Name: "TestRemovePanicService supervisor",
			Log: func(msg interface{}) {
				t.Log("supervisor log (panic bug):", msg)
			},
		},
	}
	svc1 := quickpanicservice{id: 1}
	supervisor.Add(&svc1)
	svc2 := waitservice{id: 2}
	supervisor.Add(&svc2)
	svc3 := holdingservice{id: 3}
	svc3.Add(1)
	supervisor.Add(&svc3)

	ctx, cancel := contextWithTimeout(30 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()
	svc3.Wait()

	supervisor.Remove(svc1.String())
	cancel()
	wg.Wait()

	svcs := supervisor.Services()
	if _, ok := svcs[svc1.String()]; ok {
		t.Errorf("%s should have been removed.", &svc1)
	}
}

func TestFailing(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	supervisor := Supervisor{
		Name: "TestFailing supervisor",
		Log: func(msg interface{}) {
			t.Log("supervisor log (failing):", msg)
		},
	}
	svc1 := failingservice{id: 1}
	supervisor.Add(&svc1)

	ctx, _ := contextWithTimeout(3 * time.Second)
	supervisor.Serve(ctx)

	if svc1.count == 1 {
		t.Error("the failed service should have been started just once.")
	}
}

func TestAlwaysRestart(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	supervisor := Supervisor{
		Name:        "TestAlwaysRestart supervisor",
		MaxRestarts: AlwaysRestart,
		Log: func(msg interface{}) {
			t.Log("supervisor log (always restart):", msg)
		},
	}
	svc1 := failingservice{id: 1}
	supervisor.Add(&svc1)

	ctx, _ := contextWithTimeout(3 * time.Second)
	supervisor.Serve(ctx)

	if svc1.count == 1 {
		t.Error("the failed service should have been started just once.")
	}
}

func TestHaltAfterFailure(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	supervisor := Supervisor{
		Name:        "TestHaltAfterFailure supervisor",
		MaxRestarts: 1,
		Log: func(msg interface{}) {
			t.Log("supervisor log (halt after failure):", msg)
		},
	}
	svc1 := failingservice{id: 1}
	supervisor.Add(&svc1)

	ctx, _ := contextWithTimeout(1 * time.Hour)
	supervisor.Serve(ctx)

	if svc1.count != 1 {
		t.Error("the failed service should have been started just once.")
	}
}

func TestHaltAfterPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	supervisor := Supervisor{
		Name:        "TestHaltAfterPanic supervisor",
		MaxRestarts: AlwaysRestart,
		Log: func(msg interface{}) {
			t.Log("supervisor log (halt after panic):", msg)
		},
	}

	ctx, cancel := contextWithTimeout(5 * time.Second)

	svc1 := &holdingservice{id: 1}
	svc1.Add(1)
	supervisor.Add(svc1)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()
	svc1.Wait()

	svc2 := &panicabortsupervisorservice{id: 2, cancel: cancel, supervisor: &supervisor}
	supervisor.Add(svc2)

	wg.Wait()

	if svc1.count > 1 {
		t.Error("the holding service should have not been started more than once.")
	}
}

func TestTerminationAfterPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	supervisor := Supervisor{
		Name:        "TestTerminationAfterPanic supervisor",
		MaxRestarts: AlwaysRestart,
		Log: func(msg interface{}) {
			t.Log("supervisor log (termination after panic):", msg)
		},
	}
	svc1 := &triggerpanicservice{id: 1}
	supervisor.Add(svc1)
	svc2 := &holdingservice{id: 2}
	svc2.Add(1)
	supervisor.Add(svc2)

	ctx, cancel := contextWithTimeout(5 * time.Second)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()
	svc2.Wait()

	svc3 := &holdingservice{id: 3}
	svc3.Add(1)
	supervisor.Add(svc3)
	svc3.Wait()

	supervisor.Remove(svc1.String())

	svc4 := &holdingservice{id: 4}
	svc4.Add(1)
	supervisor.Add(svc4)
	svc4.Wait()

	cancel()

	wg.Wait()

	if svc1.count > 1 {
		t.Error("the panic service should have not been started more than once.")
	}
}

func TestGroupTerminationAfterPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	supervisor := Group{
		Supervisor: &Supervisor{
			Name:        "TestTerminationAfterPanicGroup supervisor",
			MaxRestarts: AlwaysRestart,
			Log: func(msg interface{}) {
				t.Log("supervisor log (termination after panic group):", msg)
			},
		},
	}
	svc1 := panicservice{id: 1}
	supervisor.Add(&svc1)
	svc2 := &holdingservice{id: 2}
	svc2.Add(1)
	supervisor.Add(svc2)

	ctx, cancel := contextWithTimeout(10 * time.Second)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()

	svc2.Wait()
	supervisor.Remove(svc1.String())

	svc3 := &holdingservice{id: 3}
	svc3.Add(1)
	supervisor.Add(svc3)
	svc3.Wait()

	svc4 := panicservice{id: 4}
	supervisor.Add(&svc4)
	svc5 := &holdingservice{id: 5}
	svc5.Add(1)
	supervisor.Add(svc5)
	svc5.Wait()

	cancel()

	wg.Wait()

	if svc1.count > 1 {
		t.Error("the panic service should have not been started more than once.")
	}
}

func TestMaxRestart(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	supervisor := &Supervisor{
		Name:        "TestMaxRestart supervisor",
		MaxRestarts: 1,
		Log: func(msg interface{}) {
			t.Log("supervisor log (max restart):", msg)
		},
	}
	svc1 := failingservice{id: 1}
	supervisor.Add(&svc1)

	ctx, _ := contextWithTimeout(10 * time.Second)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()

	wg.Wait()

	if svc1.count > 1 {
		t.Error("the panic service should have not been started more than once.")
	}
}

func TestGroupMaxRestart(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	supervisor := Group{
		Supervisor: &Supervisor{
			Name:        "TestMaxRestartGroup supervisor",
			MaxRestarts: 1,
			Log: func(msg interface{}) {
				t.Log("supervisor log (max restart group):", msg)
			},
		},
	}
	svc1 := failingservice{id: 1}
	supervisor.Add(&svc1)

	ctx, _ := contextWithTimeout(10 * time.Second)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()

	wg.Wait()

	if svc1.count > 1 {
		t.Error("the panic service should have not been started more than once.")
	}
}

func TestAddServiceAfterServe(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor
	supervisor.Name = "TestAddServiceAfterServe supervisor"
	svc1 := &holdingservice{id: 1}
	svc1.Add(1)
	supervisor.Add(svc1)

	ctx, cancel := contextWithTimeout(2 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()

	svc1.Wait()

	svc2 := &holdingservice{id: 2}
	svc2.Add(1)
	supervisor.Add(svc2)
	svc2.Wait()

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
	supervisor.Name = "TestRemoveServiceAfterServe supervisor"
	svc1 := &holdingservice{id: 1}
	svc1.Add(1)
	supervisor.Add(svc1)
	svc2 := &holdingservice{id: 2}
	svc2.Add(1)
	supervisor.Add(svc2)

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

	svc1.Wait()
	svc2.Wait()

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
	supervisor.Name = "TestRemoveServiceAfterServeBug supervisor"
	svc1 := &holdingservice{id: 1}
	svc1.Add(1)
	supervisor.Add(svc1)

	ctx, _ := contextWithTimeout(2 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()

	svc1.Wait()
	supervisor.Remove(svc1.String())
	wg.Wait()

	if svc1.count > 1 {
		t.Errorf("the removal of a service should have terminated it. It was started %v times", svc1.count)
	}
}

func TestServices(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor
	supervisor.Name = "TestServices supervisor"
	svc1 := &holdingservice{id: 1}
	svc1.Add(1)
	supervisor.Add(svc1)
	svc2 := &holdingservice{id: 2}
	svc2.Add(1)
	supervisor.Add(svc2)

	ctx, cancel := contextWithTimeout(10 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
	}()
	svc1.Wait()
	svc2.Wait()

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

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	supervisor := Supervisor{
		Name: "TestManualCancelation supervisor",
		Log: func(msg interface{}) {
			t.Log("supervisor log (restartable):", msg)
		},
	}
	svc1 := &holdingservice{id: 1}
	svc1.Add(1)
	supervisor.Add(svc1)
	svc2 := restartableservice{id: 2, restarted: make(chan struct{})}
	supervisor.Add(&svc2)

	ctx, cancel := contextWithTimeout(10 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()

	svc1.Wait()

	// Testing restart
	<-svc2.restarted
	svcs := supervisor.Cancelations()
	svcancel := svcs[svc2.String()]
	svcancel()
	<-svc2.restarted

	cancel()
	<-ctx.Done()
	wg.Wait()
}

func TestServiceList(t *testing.T) {
	t.Parallel()

	var supervisor Supervisor
	supervisor.Name = "TestServiceList supervisor"
	svc1 := holdingservice{id: 1}
	svc1.Add(1)
	supervisor.Add(&svc1)

	ctx, cancel := contextWithTimeout(10 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
	}()
	svc1.Wait()

	svcs := supervisor.Services()
	if svc, ok := svcs[svc1.String()]; !ok || &svc1 != svc.(*holdingservice) {
		t.Errorf("could not find service when listing them. %s missing", svc1.String())
	}

	cancel()
	<-ctx.Done()
	wg.Done()
}

func TestGroup(t *testing.T) {
	t.Parallel()

	supervisor := Group{
		Supervisor: &Supervisor{
			Name: "TestGroup supervisor",
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
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("unexpected panic: %v", r)
			}
		}()
		defer wg.Done()

		supervisor.Serve(ctx)
		if !(svc1.count == svc2.count && svc1.count == 2) {
			t.Errorf("affected service of a group should affect all. svc1.count: %d svc2.count: %d (both should be 2)", svc1.count, svc2.count)
		}

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

func TestInvalidGroup(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("defer called, but not because of panic")
		}
	}()
	var group Group
	ctx, _ := contextWithTimeout(10 * time.Second)
	group.Serve(ctx)
	t.Error("this group is invalid and should have had panic()'d")
}

func TestSupervisorAbortRestart(t *testing.T) {
	t.Parallel()
	supervisor := Supervisor{
		Name: "TestAbortRestart supervisor",
		Log: func(msg interface{}) {
			t.Log("supervisor log (abort restart):", msg)
		},
	}

	svc1 := &holdingservice{id: 1}
	svc1.Add(1)
	supervisor.Add(svc1)
	svc2 := &restartableservice{id: 2}
	supervisor.Add(svc2)

	ctx, cancel := contextWithTimeout(10 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()
	svc1.Wait()

	svc3 := &holdingservice{id: 3}
	svc3.Add(1)
	supervisor.Add(svc3)
	svc3.Wait()

	cancel()
	wg.Wait()

	if svc2.count == 1 {
		t.Error("the restartable service should have been started more than once.")
	}
}

func TestTerminationAbortRestart(t *testing.T) {
	t.Parallel()
	supervisor := Supervisor{
		Name: "TestTerminationAbortRestart supervisor",
		Log: func(msg interface{}) {
			t.Log("supervisor log (termination abort restart):", msg)
		},
	}

	svc1 := &holdingservice{id: 1}
	svc1.Add(1)
	supervisor.Add(svc1)
	svc2 := &failingservice{id: 2}
	supervisor.Add(svc2)

	ctx, cancel := contextWithTimeout(10 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()

	svc1.Wait()
	supervisor.Remove(svc2.String())

	svc3 := &holdingservice{id: 3}
	svc3.Add(1)
	supervisor.Add(svc3)
	svc3.Wait()

	cancel()
	wg.Wait()

	if svc2.count == 0 {
		t.Error("the failing service should have been started at least once.")
	}
}

func TestTemporaryService(t *testing.T) {
	t.Parallel()
	supervisor := Supervisor{
		Name: "TestTemporaryService supervisor",
		Log: func(msg interface{}) {
			t.Log("supervisor log (termination abort restart):", msg)
		},
	}

	svc1 := &temporaryservice{id: 1}
	supervisor.AddService(svc1, Temporary)
	svc2 := &holdingservice{id: 2}
	svc2.Add(1)
	supervisor.Add(svc2)

	ctx, cancel := contextWithTimeout(10 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()
	svc2.Wait()

	svc3 := &holdingservice{id: 3}
	svc3.Add(1)
	supervisor.Add(svc3)
	svc3.Wait()

	cancel()
	wg.Wait()

	if svc1.count != 1 {
		t.Error("the temporary service should have been started just once.", svc1.count)
	}
}

func TestTransientService(t *testing.T) {
	t.Parallel()
	supervisor := Supervisor{
		Name: "TestTemporaryService supervisor",
		Log: func(msg interface{}) {
			t.Log("supervisor log (termination abort restart):", msg)
		},
	}

	svc1 := &transientservice{id: 1}
	supervisor.AddService(svc1, Transient)
	svc2 := &holdingservice{id: 2}
	svc2.Add(1)
	supervisor.Add(svc2)

	ctx, cancel := contextWithTimeout(10 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve(ctx)
		wg.Done()
	}()
	svc2.Wait()

	svc3 := &holdingservice{id: 3}
	svc3.Add(1)
	supervisor.Add(svc3)
	svc3.Wait()

	cancel()
	wg.Wait()

	if svc1.count != 2 {
		t.Error("the transient service should have been started just twice.", svc1.count)
	}
}

func getServiceCount(s *Supervisor) int {
	s.mu.Lock()
	l := len(s.services)
	s.mu.Unlock()
	return l
}
