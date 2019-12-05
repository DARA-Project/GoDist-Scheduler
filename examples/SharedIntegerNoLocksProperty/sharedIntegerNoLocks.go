package main

import (
	"fmt"
	"time"
    "runtime" // Import for DaraLog to report names of variables + values of events
)

const LOOPS = 50

var SharedVariable int

func main() {
    // Creates a goroutine that performs the add function 50 times
	go op(add)
    // Creates a goroutine that performs the sub function 50 times
	go op(sub)
    // Creates a goroutine that performs the mul function 50 times
	go op(mult)
    // Sleeps for 10s
	time.Sleep(10 * time.Second)
    // Prints the final value of SharedVariable.
	fmt.Println("---------Final value was",SharedVariable)
}

func op(oper func(int, int)) {
	for i:=1;i<LOOPS;i++{
		oper(1, i)
	}
}

// Adds the value 'n' to the SharedVariable variable then sleeps for 1ms
func add(n int, i int) {
	SharedVariable += n
	fmt.Println("--------Iteration", i," Add value is :", SharedVariable)
    runtime.DaraLog("Add", "main.SharedVariable", SharedVariable)
	time.Sleep(time.Millisecond)
}

// Subtracts the value 'n' from the SharedVariable variable then sleeps for 1ms
func sub(n int, i int) {
	SharedVariable -= n
	fmt.Println("--------Iteration", i, " Sub value is :", SharedVariable)
    runtime.DaraLog("Sub", "main.SharedVariable", SharedVariable)
	time.Sleep(time.Millisecond)
}

// Multiplies the SharedVariable variable with the value 'n' then sleeps for 1ms
func mult(n int, i int) {
	SharedVariable *= n
    fmt.Println("--------Iteration", i, " Mult value is :", SharedVariable)
    runtime.DaraLog("Mult", "main.SharedVariable", SharedVariable)
	time.Sleep(time.Millisecond)
}

