// +build go1.7

package supervisor

import (
	"context"
	"time"
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
