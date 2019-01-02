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
	"sync"
	"testing"

	supervisor "cirello.io/supervisor/easy"
)

func TestInvalidContext(t *testing.T) {
	ctx := context.Background()
	_, err := supervisor.Add(ctx, func(context.Context) {})
	if err != supervisor.ErrNoSupervisorAttached {
		t.Errorf("ErrNoSupervisorAttached not found: %v", err)
	}

	if err := supervisor.Remove(ctx, "fake name"); err != supervisor.ErrNoSupervisorAttached {
		t.Errorf("ErrNoSupervisorAttached not found: %v", err)
	}
}

func TestLogger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var got string
	ctx = supervisor.WithContext(ctx, supervisor.WithLogger(func(a ...interface{}) {
		if got == "" {
			got = fmt.Sprint(a...)
		}
	}))

	var wg sync.WaitGroup
	wg.Add(1)
	supervisor.Add(ctx, func(context.Context) { wg.Done() }, supervisor.Transient)
	wg.Wait()

	const expected = "function service 1 starting"
	if got != expected {
		t.Error("unexpected logged message found. got:", got, "expected:", expected)
	}
}
