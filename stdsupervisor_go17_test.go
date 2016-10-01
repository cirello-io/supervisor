// +build go1.7

package supervisor

import (
	"context"
	"time"
)

func ExampleServeContext() {
	svc := simpleservice(1)
	Add(&svc)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	ServeContext(ctx)

	// If ServeContext() runs on background, this supervisor can be halted
	// through cancel().
	cancel()
}
