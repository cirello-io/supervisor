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

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.Serve(ctx)
}
