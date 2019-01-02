// Copyright 2019 github.com/ucirello and https://cirello.io. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to writing, software distributed
// under the License is distributed on a "AS IS" BASIS, WITHOUT WARRANTIES OR
// CONDITIONS OF ANY KIND, either express or implied.
//
// See the License for the specific language governing permissions and
// limitations under the License.

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
