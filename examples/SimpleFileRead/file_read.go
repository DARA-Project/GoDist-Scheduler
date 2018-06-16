package main

import (
	"fmt"
	"log"
	"os"
)

func foo(c chan int) {
	for i := 0; i < 5; i++{
		fmt.Println("Opening file now")
		f, err := os.Open("file.txt")
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Reading file now")
		b1 := make([]byte, 20)
		n1, err := f.Read(b1)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d bytes: %s\n", n1, string(b1[:n1]))
		f.Close()
	}
	c <- 1
}

func main() {
	c := make(chan int)
	go foo(c)
	x := <-c
	log.Println(x)
}
