package easy_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	supervisor "cirello.io/supervisor/easy"
)

func Example() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	ctx = supervisor.WrapContext(ctx)
	wg.Add(1)
	supervisor.Add(ctx, func(ctx context.Context) {
		defer wg.Done()
		fmt.Println("executed successfully")
		cancel()
	})

	wg.Wait()

	// Output:
	// executed successfully
}
