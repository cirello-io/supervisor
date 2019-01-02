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

package supervisor

import (
	"context"
	"fmt"
	"sync"
)

var (
	universalFuncSvcMu sync.Mutex
	universalFuncSvc   uint64
)

func funcSvcID() uint64 {
	universalFuncSvcMu.Lock()
	universalFuncSvc++
	v := universalFuncSvc
	universalFuncSvcMu.Unlock()
	return v
}

type funcsvc struct {
	id uint64
	f  func(context.Context)
}

func (a funcsvc) Serve(ctx context.Context) {
	a.f(ctx)
}

func (a funcsvc) String() string {
	return fmt.Sprintf("function service %d", a.id)
}
