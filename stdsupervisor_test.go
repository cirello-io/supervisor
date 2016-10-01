package supervisor

import (
	"sync"
	"testing"
	"time"
)

func ExampleServe() {
	svc := simpleservice(1)
	Add(&svc)
	Serve()
}

func ExampleServeGroup() {
	svc1 := simpleservice(1)
	Add(&svc1)
	svc2 := simpleservice(2)
	Add(&svc2)
	ServeGroup()
}

func TestDefaultSupevisor(t *testing.T) {
	t.Parallel()

	ctx, cancel := contextWithTimeout(10 * time.Second)
	defaultContext = ctx
	svc := waitservice{id: 1}

	Add(&svc)
	if len(DefaultSupervisor.services) != 1 {
		t.Errorf("%s should have been added", svc.String())
	}

	Remove(svc.String())
	if len(DefaultSupervisor.services) != 0 {
		t.Errorf("%s should have been removed. services: %#v", svc.String(), DefaultSupervisor.services)
	}

	Add(&svc)

	svcs := Services()
	if _, ok := svcs[svc.String()]; !ok {
		t.Errorf("%s should have been found", svc.String())
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		Serve()
		wg.Done()
	}()
	<-DefaultSupervisor.started

	cs := Cancelations()
	if _, ok := cs[svc.String()]; !ok {
		t.Errorf("%s's cancelation should have been found", svc.String())
	}

	cancel()
	wg.Wait()
}

func TestServeGroup(t *testing.T) {
	t.Parallel()

	ctx, cancel := contextWithTimeout(10 * time.Second)
	defaultContext = ctx
	svc := waitservice{id: 1}

	Add(&svc)
	if len(DefaultSupervisor.services) != 1 {
		t.Errorf("%s should have been added", svc.String())
	}

	Remove(svc.String())
	if len(DefaultSupervisor.services) != 0 {
		t.Errorf("%s should have been removed. services: %#v", svc.String(), DefaultSupervisor.services)
	}

	Add(&svc)

	svcs := Services()
	if _, ok := svcs[svc.String()]; !ok {
		t.Errorf("%s should have been found", svc.String())
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		ServeGroup()
		wg.Done()
	}()
	<-DefaultSupervisor.started

	cs := Cancelations()
	if _, ok := cs[svc.String()]; !ok {
		t.Errorf("%s's cancelation should have been found", svc.String())
	}

	cancel()
	wg.Wait()
}
