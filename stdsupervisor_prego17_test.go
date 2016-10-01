// +build !go1.7

package supervisor

import (
	"time"

	"golang.org/x/net/context"
)

func ExampleServeContext() {
	svc := Simpleservice(1)
	Add(&svc)

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	ServeContext(ctx)
}
