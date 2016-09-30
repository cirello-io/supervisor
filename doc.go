/*
Package supervisor provides supervisor trees for Go applications.

This package is a clean reimplementation of github.com/thejerf/suture, aiming
to be more Go idiomatic, thus less Erlang-like.

It is built on top of context package, with all of its advantages, namely the
possibility trickle down context-related values and cancellation signals.

TheJerf's blog post about Suture is a very good and helpful read to understand
how this package has been implemented.

http://www.jerf.org/iri/post/2930

Example:
	package main

	import (
		"fmt"
		"os"
		"os/signal"
		"time"

		"cirello.io/supervisor"
		"golang.org/x/net/context"
	)

	type Simpleservice int

	func (s *Simpleservice) String() string {
		return fmt.Sprintf("simple service %d", int(*s))
	}

	func (s *Simpleservice) Serve(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				fmt.Println("do something...")
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	func main(){
		svc := Simpleservice(1)
		supervisor.Add(&svc)

		// Simply, if not special context is needed:
		// supervisor.Serve()
		// Or, using context.Context to propagate behavior:
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		ctx, cancel := context.WithCancel(context.Background())
		go func(){
			<-c
			fmt.Println("halting supervisor...")
			cancel()
		}()
		supervisor.ServeContext(ctx)
	}
*/
package supervisor // import "cirello.io/supervisor"
