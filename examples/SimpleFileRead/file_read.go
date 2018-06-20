package main

import (
	"fmt"
	"log"
	"os"
//	"time"
)

func bar(c chan int) {
	for i := 0; i < 5; i++{
		fmt.Println("Opening file2 now")
		f, err := os.Open("file2.txt")
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Reading file2 now")
		b1 := make([]byte, 20)
		n1, err := f.Read(b1)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d bytes of file2: %s\n", n1, string(b1[:n1]))
		f.Close()
//		time.Sleep(time.Millisecond)
	}
	c <- 1

}

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
//		time.Sleep(time.Millisecond)
	}
	c <- 1
}

func main() {
	c := make(chan int)
	c2 := make(chan int)
	go foo(c)
	go bar(c2)
	x := <-c
	y := <-c2
	log.Println(x)
	log.Println(y)
}
