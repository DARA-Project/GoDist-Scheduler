package main

import (
    "fmt"
)

var m map[string]string
var comm chan int

func initmap() {
    m = make(map[string]string)
    comm <- 1
}

func reader() {
    for i := 0; i <= 5; i++ {
        for k, v := range m {
            fmt.Println(k,v)
        }
    }
    comm <- 2
}

func writer() {
    for i := 0; i <= 5; i++ {
        m[string(i)] = string(i)
    }
    comm <- 3
}

func main() {
    // Initializes the communication channel between the main
    // goroutine and the children goroutines
    comm = make(chan int, 3)
    // Launches a goroutine that initializes the shared map
    go initmap()
    // Launches a goroutine that reads from the shared map
    go reader()
    // Launches a goroutine that writes to the shared map
    go writer()
    <- comm
    <- comm
    <- comm
}
