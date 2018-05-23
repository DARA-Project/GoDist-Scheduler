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
	time.Sleep(3 * time.Second)
	fmt.Println(shared)
}

func op(oper func(int)) {
	for i:=1;i<LOOPS;i++{
		oper(i)
	}
}

func add(n int) {
	shared += n
	fmt.Println(shared)
	time.Sleep(time.Millisecond)
}

func sub(n int) {
	shared -= n
	fmt.Println(shared)
	time.Sleep(time.Millisecond)
}

func mult(n int) {
	shared *= n
	fmt.Println(shared)
	time.Sleep(time.Millisecond)
}

func div(n int) {
	shared/=n
	fmt.Println(shared)
	time.Sleep(time.Millisecond)
}



