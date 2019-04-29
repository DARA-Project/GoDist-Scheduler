package main

import (
    "fmt"
    "sync"
    "time"
)

const LOOPS = 50

var (
    mux sync.Mutex
    shared int
)

func main() {
    go op(add)
    go op(sub)
    time.Sleep(10 * time.Second)
    fmt.Println("----------------Final Value is :", shared)
}

func op(oper func(int, int)) {
    for i:=1;i<LOOPS;i++{
        oper(1,i)
    }
}

func add(n int, i int) {
    mux.Lock()
	shared += n
	fmt.Println("--------Iteration", i," Add value is :", shared)
    mux.Unlock()
}

func sub(n int, i int) {
    mux.Lock()
	shared -= n
	fmt.Println("--------Iteration", i, " Sub value is :", shared)
    mux.Unlock()
}
