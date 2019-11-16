package main

// This script prints out all the fully named
// variables in user-written code present in
// a go binary

import (
    "log"
    "os"
    "github.com/DARA-Project/GoDist-Scheduler/bininfo"
)

func main() {
    if len(os.Args) != 2 {
        log.Fatal("Usage: go run variable_info.go <path/to/go/binary>")
    }
    err := bininfo.PrintBinaryInfo(os.Args[1], []string{})
    if err != nil {
        log.Fatal(err)
    }
}
