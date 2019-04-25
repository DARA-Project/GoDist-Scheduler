package main

import (
	"log"
	"os"
	"time"
)

// Opens, reads, and closes "file2.txt"
// and then goes to sleep
func bar(c chan int) {
	f, err := os.Open("file2.txt")
	if err != nil {
		log.Fatal(err)
	}

	b1 := make([]byte, 20)
	_, err = f.Read(b1)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()
	time.Sleep(time.Millisecond)
	c <- 1

}

// Opens, reads, and closes "file.txt"
// and then goes to sleep
func foo(c chan int) {
	f, err := os.Open("file.txt")
	if err != nil {
		log.Fatal(err)
	}

	b1 := make([]byte, 20)
	_, err = f.Read(b1)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()
	time.Sleep(time.Millisecond)
	c <- 1
}

func main() {

	c := make(chan int)
	c2 := make(chan int)
	go foo(c)
	go bar(c2)
	x := <-c
	y := <-c2
	log.Println("Got x", x)
	log.Println("Got y", y)

}
