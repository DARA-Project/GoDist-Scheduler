package main

import (
	//"fmt"
	"log"
	"os"
//	"time"
)

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
	normal()
}
