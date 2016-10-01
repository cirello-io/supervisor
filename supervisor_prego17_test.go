// +build !go1.7

package supervisor

import (
	"time"

	"golang.org/x/net/context"
)

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
