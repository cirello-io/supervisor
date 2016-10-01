// +build go1.7

package supervisor

import (
	"context"
	"time"
)

func ExampleSupervisor() {
	var supervisor Supervisor

	svc := Simpleservice(1)
	supervisor.Add(&svc)

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.Serve(ctx)
}
