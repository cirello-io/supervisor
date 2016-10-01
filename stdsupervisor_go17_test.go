// +build go1.7

package supervisor

import (
	"context"
	"time"
)

func ExampleServeContext() {
	svc := Simpleservice(1)
	Add(&svc)

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	ServeContext(ctx)
}
