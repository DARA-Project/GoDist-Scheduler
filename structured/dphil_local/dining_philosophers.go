package main

import "runtime"

import (
	"fmt"
	"math/rand"
	"time"
)

type Philosopher struct {
	name		string
	chopstick	chan bool
	neighbor	*Philosopher
}

func makePhilosopher(name string, neighbor *Philosopher) *Philosopher {
	runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:15:19")
	phil := &Philosopher{name, make(chan bool, 1), neighbor}
	phil.chopstick <- true
	return phil
}

func (phil *Philosopher) think() {
	runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:21:24")
	fmt.Printf("%v is thinking.\n", phil.name)
	time.Sleep(time.Duration(rand.Int63n(1e9)))
}

func (phil *Philosopher) eat() {
	runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:26:29")
	fmt.Printf("%v is eating.\n", phil.name)
	time.Sleep(time.Duration(rand.Int63n(1e9)))
}

func (phil *Philosopher) getChopsticks() {
	runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:31:33")
	timeout := make(chan bool)
	go func() {
		runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:33:36")
		time.Sleep(1e9)
		timeout <- true
	}()
	runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:37:39")
	<-phil.chopstick
	fmt.Printf("%v got his chopstick.\n", phil.name)
	select {
	case <-phil.neighbor.chopstick:
		runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:40:43")
		fmt.Printf("%v got %v's chopstick.\n", phil.name, phil.neighbor.name)
		fmt.Printf("%v has two chopsticks.\n", phil.name)
		return
	case <-timeout:
		runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:44:47")
		phil.chopstick <- true
		phil.think()
		phil.getChopsticks()
	}
}

func (phil *Philosopher) returnChopsticks() {
	runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:51:54")
	phil.chopstick <- true
	phil.neighbor.chopstick <- true
}

func (phil *Philosopher) dine(announce chan *Philosopher) {
	runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:56:62")
	phil.think()
	phil.getChopsticks()
	phil.eat()
	phil.returnChopsticks()
	announce <- phil
}

func main() {
	runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:64:69")

	names := []string{"A", "B", "C"}
	philosophers := make([]*Philosopher, len(names))
	var phil *Philosopher
	for i, name := range names {
		runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:69:72")
		phil = makePhilosopher(name, phil)
		philosophers[i] = phil
	}
	runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:73:77")
	philosophers[0].neighbor = phil
	fmt.Printf("There are %v philosophers sitting at a table.\n", len(philosophers))
	fmt.Println("They each have one chopstick, and must borrow from their neighbor to eat.")
	announce := make(chan *Philosopher)
	for _, phil := range philosophers {
		runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:77:79")
		go phil.dine(announce)
	}
	runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:80:80")
	for i := 0; i < len(names); i++ {
		runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:80:82")
		phil := <-announce
		go func() {
			runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:82:87")

			time.Sleep(time.Second)
			close(phil.chopstick)
		}()
		runtime.ReportBlockCoverage("../structured/dphil_local/dining_philosophers.go:88:88")
		fmt.Printf("%v is done dining.\n", phil.name)
	}
}
