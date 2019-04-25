package main

import (
	"log"
	"os"
	"time"
    "strconv"
)

func normal() {
    iterations, err := strconv.Atoi(os.Getenv("ITERATIONS"))
    if err != nil {
        log.Fatal(err)
    }
    for i := 0; i < iterations; i++ {
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
}

func main() {
	normal()
    time.Sleep(10 * time.Second)
}
