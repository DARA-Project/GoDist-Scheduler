package main

import (
	"log"
	"os"
	"testing"
)

func BenchmarkFileOpen(b *testing.B) {
	f := CreateOrDie("hello_world.txt")
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.Open("hello_world.txt")
	}
	b.StopTimer()
	RemoveOrDie("hello_world.txt")

}

func BenchmarkFileRead(b *testing.B) {
	f := CreateOrDie("hello_world.txt")
	b1 := make([]byte, 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Read(b1)
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie("hello_world.txt")
}

func BenchmarkFileClose(b *testing.B) {
	f := CreateOrDie("hello_world.txt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Close()
		b.StopTimer()
		f = OpenOrDie("hello_world.txt")
		b.StartTimer()
	}
	RemoveOrDie("hello_world.txt")
}

func BenchmarkFileWrite(b *testing.B) {
	f := CreateOrDie("write_hello.txt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Write([]byte("Hello World\n"))
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie("write_hello.txt")
}

func BenchmarkFileFstat(b *testing.B) {
	f := CreateOrDie("hello_world.txt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Stat()
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie("hello_world.txt")
}

func BenchmarkFileStat(b *testing.B) {
	f := CreateOrDie("hello_world.txt")
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.Stat("hello_world.txt")
	}
	b.StopTimer()
	RemoveOrDie("hello_world.txt")
}

func BenchmarkFileLstat(b *testing.B) {
	f := CreateOrDie("hello_world.txt")
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.Lstat("hello_world.txt")
	}
	b.StopTimer()
	RemoveOrDie("hello_world.txt")
}

func BenchmarkFileLseek(b *testing.B) {
	f := CreateOrDie("hello_world.txt")
	f.Write([]byte("Hello World"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Seek(5, 0)
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie("hello_world.txt")
}

// Helpers to reduce boilerplate code

func CreateOrDie(name string) *os.File {
	f, err := os.Create(name)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

func OpenOrDie(name string) *os.File {
	f, err := os.Open(name)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

func RemoveOrDie(name string) {
	err := os.Remove(name)
	if err != nil {
		log.Fatal(err)
	}
}
