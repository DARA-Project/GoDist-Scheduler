package main

/*
This is a test of thread non-determinism; threads communicate on a
shared channel, and perform operations on a shared resource. The
purpose of this test is to demonstrate that GoDist has the ability to
accuratly replay the thread schedule of a single process which
communicates on channels

BUG This test seems to be deterministic automatically when run with
GOMAXPROCS set to 1, which makes it a bad test becuase a simplifing
assumptions about GoDist is that the execution will be serialized i.e
GOMAXPROCS will allways equal 1. This fact makes this a bad test.
*/


import (
	"log"
	"time"
)

const LOOPS = 50
const THREADS = 3

var shared int
var comm chan int

func main() {
	comm = make(chan int, THREADS)
	// Create new goroutine that runs add function 50 times
    go op(add)
	// Create new goroutine that runs sub function 50 times
	go op(sub)
	// Create new goroutine that runs mult function 50 times
	go op(mult)
	comm <- 1
	comm <- 2
	comm <- 3
	log.Println("Main thread about to sleep")
	time.Sleep(3 * time.Second)
	log.Println(shared)
}

func op(oper func(int)) {
	for i:=1;i<LOOPS;i++{
		val := <- comm
		oper(val)
		comm <- shared + i
		//time.Sleep(time.Nanosecond)
	}
}

func add(n int) {
	shared += n
	log.Println("add ->",shared)
}

func sub(n int) {
	shared -= n
	log.Println("sub ->",shared)
}

func mult(n int) {
	shared *= n
	log.Println("mult ->",shared)
}




