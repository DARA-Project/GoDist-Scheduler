package main

import "runtime"

import (
	//"fmt"
	"log"
	"os"
)

func normal() {
	runtime.ReportBlockCoverage("../examples/SimpleInstrument/file_read.go:10:12")
	f, err := os.Open("file.txt")
	if err != nil {
		runtime.ReportBlockCoverage("../examples/SimpleInstrument/file_read.go:12:14")
		log.Fatal(err)
	}
	runtime.ReportBlockCoverage("../examples/SimpleInstrument/file_read.go:16:18")

	b1 := make([]byte, 20)
	_, err = f.Read(b1)
	if err != nil {
		runtime.ReportBlockCoverage("../examples/SimpleInstrument/file_read.go:18:20")
		log.Fatal(err)
	}
	runtime.ReportBlockCoverage("../examples/SimpleInstrument/file_read.go:21:21")
	f.Close()
}

func main() {
	runtime.ReportBlockCoverage("../examples/SimpleInstrument/file_read.go:24:26")
	normal()
}
