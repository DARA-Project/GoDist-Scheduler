package main

import (
    "sync"
)

var mux sync.Mutex
var comm chan int
var comm_main chan int

func goroutine1() {
    mux.Lock()
    comm <- 1
    mux.Unlock()
}

func goroutine2() {
    for {
        mux.Lock()
        mux.Unlock()
        <- comm
    }
}

func main() {
    comm_main = make(chan int, 2)
    comm = make(chan int, 1)
    go goroutine1()
    go goroutine2()
    <- comm_main
    <- comm_main
}
