package supervisor_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cirello.io/supervisor"
)

type Simpleservice struct {
	id int
	sync.WaitGroup
}

func (s *Simpleservice) Serve(ctx context.Context) {
	fmt.Println(s.String())
	s.Done()
	<-ctx.Done()
}

func (s *Simpleservice) String() string {
	return fmt.Sprintf("simple service %d", s.id)
}

func ExampleSupervisor() {
	var supervisor supervisor.Supervisor

	svc := &Simpleservice{id: 1}
	svc.Add(1)
	supervisor.Add(svc)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	go supervisor.Serve(ctx)

	svc.Wait()
	cancel()
}

func ExampleGroup() {
	supervisor := supervisor.Group{
		Supervisor: &supervisor.Supervisor{},
	}

	svc1 := &Simpleservice{id: 1}
	svc1.Add(1)
	supervisor.Add(svc1)
	svc2 := &Simpleservice{id: 2}
	svc2.Add(1)
	supervisor.Add(svc2)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	go supervisor.Serve(ctx)

	svc1.Wait()
	svc2.Wait()
	cancel()
}
