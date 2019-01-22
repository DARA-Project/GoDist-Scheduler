package main

/*

This test is similar to sharedIntegerChanel, with the addition of
randomness. A global random seed is set from the time package, and a
random integer is added to the shared integer before it is written to
the communication channel.

The nodeterminism of the first line in main()
[rand.Seed(int64(time.Now().Nanosecond()))] is killed by a one line
change to the math/rand package where the seed of any random instance
is set to 0.

The second line of man instantiates an instance of a rand type also
using wall clock time as a seed. This nondeterminism is killed in
math/rng.go by also setting the seed of each rng to 0.

TODO's
At the time of writing, May 22 2018 the integer output has the
suffix 225. This should be turned into a test.

*/


import (
	"log"
	"time"
	"math/rand"
)

const LOOPS = 50
const THREADS = 3

var shared int
var comm chan int
var r *rand.Rand

func main() {
	rand.Seed(int64(time.Now().Nanosecond()))
	r = rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	comm = make(chan int, THREADS)
	go op(add)
	go op(sub)
	go op(mult)
	//go op(div)
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
		//Randomness
		if rand.Int() % 2 == 0 {
			comm <- shared + rand.Int()
		} else {
			comm <- shared + r.Int()
		}

		time.Sleep(time.Nanosecond)
	}
}

func add(n int) {
	shared += n
	log.Println("add ->",shared,"<------------------------------------")
}

func sub(n int) {
	shared -= n
	log.Println("sub ->",shared,"<------------------------------------")
}

func mult(n int) {
	shared *= n
	log.Println("mult ->",shared,"<------------------------------------")
}

