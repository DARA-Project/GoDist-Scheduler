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

func BenchmarkFileClose(b *testing.B) {
	f, err := os.Open("hello_world.txt")
	if err != nil {
		log.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Close()
		b.StopTimer()
		f, _ = os.Open("hello_world.txt")
		b.StartTimer()
	}
}

func BenchmarkFileWrite(b *testing.B) {
	f, err := os.Create("write_hello.txt")
	if err != nil {
		log.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = f.Write([]byte("Hello World\n"))
	}
	b.StopTimer()
	f.Close()
	err = os.Remove("write_hello.txt")
	if err != nil {
		log.Fatal(err)
	}
}
