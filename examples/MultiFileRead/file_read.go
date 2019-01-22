package main

import (
	//"fmt"
	"log"
	"os"
//	"time"
)

func bar(c chan int) {
	for i := 0; i < 5; i++{
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
//		time.Sleep(time.Millisecond)
	}
	c <- 1

}

func foo(c chan int) {
	for i := 0; i < 5; i++{
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
//		time.Sleep(time.Millisecond)
	}
	c <- 1
}

func normal() {
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

	//normal()
}
