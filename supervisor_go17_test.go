// +build go1.7

package supervisor

import (
	"context"
	"fmt"
	"sync"
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

type failingservice struct {
	id    int
	mu    sync.Mutex
	count int
}

func (s *failingservice) Serve(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
		time.Sleep(100 * time.Millisecond)
		s.mu.Lock()
		s.count++
		s.mu.Unlock()
		return
	}
}

func (s *failingservice) String() string {
	return fmt.Sprintf("failing service %v", s.id)
}

func (s *failingservice) Count() int {
	s.mu.Lock()
	c := s.count
	s.mu.Unlock()
	return c
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
	id    int
	mu    sync.Mutex
	count int
}

func (s *panicservice) Serve(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(100 * time.Millisecond)
			s.mu.Lock()
			s.count++
			s.mu.Unlock()
			panic("forcing panic")
		}
	}
}

func (s *panicservice) String() string {
	return fmt.Sprintf("panic service %v", s.id)
}

func (s *panicservice) Count() int {
	s.mu.Lock()
	c := s.count
	s.mu.Unlock()
	return c
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
	mu        sync.Mutex
	count     int
}

func (s *restartableservice) Serve(ctx context.Context) {
	for {
		s.mu.Lock()
		s.count++
		s.mu.Unlock()
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

func (s *restartableservice) Count() int {
	s.mu.Lock()
	c := s.count
	s.mu.Unlock()
	return c
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

func (s *waitservice) Count() int {
	s.mu.Lock()
	c := s.count
	s.mu.Unlock()
	return c
}

type temporaryservice struct {
	id    int
	mu    sync.Mutex
	count int
}

func (s *temporaryservice) Serve(ctx context.Context) {
	s.mu.Lock()
	s.count++
	s.mu.Unlock()
}

func (s *temporaryservice) Count() int {
	s.mu.Lock()
	c := s.count
	s.mu.Unlock()
	return c
}

func (s *temporaryservice) String() string {
	return fmt.Sprintf("temporary service %v", s.id)
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

type transientservice struct {
	id    int
	mu    sync.Mutex
	count int
	sync.WaitGroup
}

func (s *transientservice) Serve(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count++
	if s.count == 1 {
		panic("panic once")
	}
	s.Done()
	<-ctx.Done()
}

func (s *transientservice) String() string {
	return fmt.Sprintf("transient service %v", s.id)
}

type triggerfailservice struct {
	id        int
	trigger   chan struct{}
	listening chan struct{}
	log       func(msg string, args ...interface{})
	count     int
}

func (s *triggerfailservice) Serve(ctx context.Context) {
	s.listening <- struct{}{}
	s.log("listening %d", s.id)
	select {
	case <-s.trigger:
		s.log("triggered %d", s.id)
		s.count++
	case <-ctx.Done():
		s.log("context done %d", s.id)
		s.count++
	}
}

func (s *triggerfailservice) String() string {
	return fmt.Sprintf("trigger fail service %v", s.id)
}
