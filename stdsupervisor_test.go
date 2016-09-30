package supervisor

import (
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"
)

func TestDefaultSupevisor(t *testing.T) {
	t.Parallel()

	var cancel context.CancelFunc
	defaultContext, cancel = context.WithTimeout(defaultContext, 10*time.Second)
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
