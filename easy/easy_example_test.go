package easy_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	supervisor "cirello.io/supervisor/easy"
)

func Example() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	ctx = supervisor.WithContext(ctx)
	wg.Add(1)
	serviceName, err := supervisor.Add(ctx, func(ctx context.Context) {
		select {
		case <-ctx.Done():
			return
		default:
			defer wg.Done()
			fmt.Println("executed successfully")
			cancel()
		}
	})
	if err != nil {
		log.Fatal(err)
	}

	wg.Wait()

	if err := supervisor.Remove(ctx, serviceName); err != nil {
		log.Fatal(err)
	}

	// Output:
	// executed successfully
}
