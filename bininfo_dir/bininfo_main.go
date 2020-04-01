package main

import (
	"github.com/DARA-Project/GoDist-Scheduler/bininfo"
	"log"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: go run bininfo_main.go <path/to/go/binary>")
	}
	err := bininfo.PrintBinaryInfo(os.Args[1], []string{})
	if err != nil {
		log.Fatal(err)
	}
}
