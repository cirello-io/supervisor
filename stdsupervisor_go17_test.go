// +build go1.7

package supervisor_test

import (
	"context"
	"fmt"
	"time"

	"cirello.io/supervisor"
)

type Simpleservice int

func (s *Simpleservice) Serve(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (s *Simpleservice) String() string {
	return fmt.Sprintf("simple service %d", int(*s))
}

func ExampleServeContext() {
	svc := Simpleservice(1)
	supervisor.Add(&svc)

	supervisor.ServeContext(context.Background())
}
