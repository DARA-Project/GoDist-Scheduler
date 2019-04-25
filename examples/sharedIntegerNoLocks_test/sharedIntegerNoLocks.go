package main

import (
	"fmt"
	"time"
)

const LOOPS = 50

var shared int

func main() {
    // Creates a goroutine that performs the add function 50 times
	go op(add)
    // Creates a goroutine that performs the sub function 50 times
	go op(sub)
    // Creates a goroutine that performs the mul function 50 times
	go op(mult)
    // Sleeps for 10s
	time.Sleep(10 * time.Second)
    // Prints the final value of shared.
	fmt.Println("---------Final value was",shared)
}

func op(oper func(int, int)) {
	for i:=1;i<LOOPS;i++{
		oper(1, i)
	}
}

// Adds the value 'n' to the shared variable then sleeps for 1ms
func add(n int, i int) {
	shared += n
	fmt.Println("--------Iteration", i," Add value is :", shared)
	time.Sleep(time.Millisecond)
}

// Subtracts the value 'n' from the shared variable then sleeps for 1ms
func sub(n int, i int) {
	shared -= n
	fmt.Println("--------Iteration", i, " Sub value is :", shared)
	time.Sleep(time.Millisecond)
}

// Multiplies the shared variable with the value 'n' then sleeps for 1ms
func mult(n int, i int) {
	shared *= n
    fmt.Println("--------Iteration", i, " Mult value is :", shared)
	time.Sleep(time.Millisecond)
}

