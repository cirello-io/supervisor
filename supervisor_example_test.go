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

package supervisor_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cirello.io/supervisor"
)

type Simpleservice struct {
	id int
	sync.WaitGroup
}

func (s *Simpleservice) Serve(ctx context.Context) {
	fmt.Println(s.String())
	s.Done()
	<-ctx.Done()
}

func (s *Simpleservice) String() string {
	return fmt.Sprintf("simple service %d", s.id)
}

func ExampleSupervisor() {
	var supervisor supervisor.Supervisor

	svc := &Simpleservice{id: 1}
	svc.Add(1)
	supervisor.Add(svc)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	go supervisor.Serve(ctx)

	svc.Wait()
	cancel()
}

func ExampleGroup() {
	supervisor := supervisor.Group{
		Supervisor: &supervisor.Supervisor{},
	}

	svc1 := &Simpleservice{id: 1}
	svc1.Add(1)
	supervisor.Add(svc1)
	svc2 := &Simpleservice{id: 2}
	svc2.Add(1)
	supervisor.Add(svc2)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	go supervisor.Serve(ctx)

	svc1.Wait()
	svc2.Wait()
	cancel()
}
