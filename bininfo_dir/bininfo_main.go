package main

import (
    "log"
    "os"
    "github.com/DARA-Project/GoDist-Scheduler/bininfo"
)

func main() {
    err := bininfo.PrintBinaryInfo(os.Args[1], []string{})
    if err != nil {
        log.Fatal(err)
    }
}
