// +build go1.7

package supervisor

import (
	"context"
	"fmt"
	"time"
)

type simpleservice int

func (s *simpleservice) Serve(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (s *simpleservice) String() string {
	return fmt.Sprintf("simple service %d", int(*s))
}

func ExampleSupervisor() {
	var supervisor Supervisor

	svc := simpleservice(1)
	supervisor.Add(&svc)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.Serve(ctx)

	// If Serve() runs on background, this supervisor can be halted through
	// cancel().
	cancel()
}

func ExampleGroup() {
	var supervisor Group

	svc1 := simpleservice(1)
	supervisor.Add(&svc1)
	svc2 := simpleservice(2)
	supervisor.Add(&svc2)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.Serve(ctx)

	// If Serve() runs on background, this supervisor can be halted through
	// cancel().
	cancel()
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

func (s *waitservice) Serve(ctx context.Context) {
	s.mu.Lock()
	s.count++
	s.mu.Unlock()
	<-ctx.Done()
}

func (s *holdingservice) Serve(ctx context.Context) {
	s.mu.Lock()
	s.count++
	s.mu.Unlock()
	s.Done()
	<-ctx.Done()
}
