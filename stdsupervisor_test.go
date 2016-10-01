package supervisor

import (
	"sync"
	"testing"
)

func TestDefaultSupevisorAndGroup(t *testing.T) {
	svc := &holdingservice{id: 1}
	svc.Add(1)

	Add(svc)
	if len(DefaultSupervisor.services) != 1 {
		t.Errorf("%s should have been added", svc.String())
	}

	Remove(svc.String())
	if len(DefaultSupervisor.services) != 0 {
		t.Errorf("%s should have been removed. services: %#v", svc.String(), DefaultSupervisor.services)
	}

	Add(svc)

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

	svc.Wait()

	cs := Cancelations()
	if _, ok := cs[svc.String()]; !ok {
		t.Errorf("%s's cancelation should have been found. %#v", svc.String(), cs)
	}

	Cancel()
	wg.Wait()

	svc.Add(1)
	Add(svc)
	if len(DefaultSupervisor.services) != 1 {
		t.Errorf("%s should have been added", svc.String())
	}

	wg.Add(1)
	go func() {
		ServeGroup()
		wg.Done()
	}()

	svc.Wait()
	Cancel()
	wg.Wait()
}
