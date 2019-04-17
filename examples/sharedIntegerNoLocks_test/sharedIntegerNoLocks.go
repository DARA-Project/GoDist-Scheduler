package main

import (
	"fmt"
	"time"
	//"math/rand"
)

const LOOPS = 50

var shared int

func main() {
	go op(add)
	go op(sub)
	go op(mult)
	//go op(div)
	time.Sleep(10 * time.Second)
	fmt.Println("---------Final value was",shared)
}

func op(oper func(int, int)) {
	for i:=1;i<LOOPS;i++{
		oper(1, i)
	}
}

func add(n int, i int) {
	shared += n
	fmt.Println("--------Iteration", i," Add value is :", shared)
	time.Sleep(time.Millisecond)
}

func sub(n int, i int) {
	shared -= n
	fmt.Println("--------Iteration", i, " Sub value is :", shared)
	time.Sleep(time.Millisecond)
}

func mult(n int, i int) {
	shared *= n
    fmt.Println("--------Iteration", i, " Mult value is :", shared)
	time.Sleep(time.Millisecond)
}

func div(n int, i int) {
	shared/=n
	fmt.Println(shared)
	time.Sleep(time.Millisecond)
}



