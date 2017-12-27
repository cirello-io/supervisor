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
