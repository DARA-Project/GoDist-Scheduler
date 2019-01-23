package main

import (
	"log"
	"os"
	"testing"
)

func BenchmarkFileOpen(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Open("hello_world.txt")
	}
}

func BenchmarkFileRead(b *testing.B) {
	f, err := os.Open("hello_world.txt")
	if err != nil {
		log.Fatal(err)
	}
	b1 := make([]byte, 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Read(b1)
	}
}
