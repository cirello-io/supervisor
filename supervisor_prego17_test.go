// +build !go1.7

package supervisor

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/net/context"
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

type holdingservice struct {
	id    int
	mu    sync.Mutex
	count int
	sync.WaitGroup
}

func (s *holdingservice) Serve(ctx context.Context) {
	s.mu.Lock()
	s.count++
	s.mu.Unlock()
	s.Done()
	<-ctx.Done()
}

func (s *holdingservice) String() string {
	return fmt.Sprintf("holding service %v", s.id)
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

type quickpanicservice struct {
	id, count int
}

func (s *quickpanicservice) Serve(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			s.count++
			panic("forcing panic")
		}
	}
}

func (s *quickpanicservice) String() string {
	return fmt.Sprintf("panic service %v", s.id)
}

type restartableservice struct {
	id        int
	restarted chan struct{}
	count     int
}

func (s *restartableservice) Serve(ctx context.Context) {
	for {
		s.count++
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
	return fmt.Sprintf("restartable service %v", s.id)
}

type waitservice struct {
	id    int
	mu    sync.Mutex
	count int
}

func (s *waitservice) Serve(ctx context.Context) {
	s.mu.Lock()
	s.count++
	s.mu.Unlock()
	<-ctx.Done()
}

func (s *waitservice) String() string {
	return fmt.Sprintf("wait service %v", s.id)
}

type triggerpanicservice struct {
	id, count int
}

func (s *triggerpanicservice) Serve(ctx context.Context) {
	<-ctx.Done()
	s.count++
	panic("forcing panic")
}

func (s *triggerpanicservice) String() string {
	return fmt.Sprintf("iterative panic service %v", s.id)
}

type panicabortsupervisorservice struct {
	id         int
	cancel     context.CancelFunc
	supervisor *Supervisor
}

func (s *panicabortsupervisorservice) Serve(ctx context.Context) {
	s.supervisor.mu.Lock()
	defer s.supervisor.mu.Unlock()
	s.supervisor.terminations = nil
	s.cancel()
	panic("forcing panic")
}

func (s *panicabortsupervisorservice) String() string {
	return fmt.Sprintf("super panic service %v", s.id)
}
