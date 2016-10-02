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

type restartableservice struct {
	id        int
	restarted chan struct{}
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

func (s *restartableservice) String() string {
	return fmt.Sprintf("restartable service %v", *s)
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
