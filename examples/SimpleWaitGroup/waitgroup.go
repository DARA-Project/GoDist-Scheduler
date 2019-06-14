package main

import (
    "fmt"
    "sync"
    "time"
)

func main() {
    var myWaitGroup sync.WaitGroup

    myWaitGroup.Add(2) // Must wait for 2 calls to 'done' before moving on

    go func(w *sync.WaitGroup) {
        fmt.Println("Start goroutine 1")
        fmt.Println("End goroutine 1")
        w.Done()
    }(&myWaitGroup)

    go func(w *sync.WaitGroup) {
        fmt.Println("Start goroutine 2")
        time.Sleep(time.Second * 3)
        fmt.Println("End goroutine 2")
        w.Done()
    }(&myWaitGroup)

    fmt.Println("Waiting for all goroutines to exit")
    myWaitGroup.Wait()
    fmt.Println("Waited for all goroutines to exit")
}
