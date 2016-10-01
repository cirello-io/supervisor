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
	fmt.Println(s.String())
	<-ctx.Done()
}

func (s *Simpleservice) String() string {
	return fmt.Sprintf("simple service %d", int(*s))
}

func ExampleServeContext() {
	svc := Simpleservice(1)
	supervisor.Add(&svc)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	supervisor.ServeContext(ctx)
	cancel()

	// output:
	// simple service 1
}
