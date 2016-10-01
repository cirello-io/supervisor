package supervisor_test

import (
	"time"

	"cirello.io/supervisor"
)

func ExampleServe() {
	supervisor.DefaultSupervisor.Name = "ExampleServe"
	svc := Simpleservice(1)
	supervisor.Add(&svc)

	go supervisor.Serve()

	time.Sleep(1 * time.Second)
	supervisor.Cancel()

	// output:
	// simple service 1
}

func ExampleServeGroup() {
	supervisor.DefaultSupervisor.Name = "ExampleServeGroup"
	svc1 := Simpleservice(1)
	supervisor.Add(&svc1)
	svc2 := Simpleservice(2)
	supervisor.Add(&svc2)

	go supervisor.ServeGroup()

	time.Sleep(3 * time.Second)
	supervisor.Cancel()

	// unordered output:
	// simple service 1
	// simple service 2
}
