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
	f.Write([]byte("Hello World\n"))
	f.Close()
	f = OpenOrDie("hello_world.txt")
	b1 := make([]byte, 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		f.Read(b1)
		b.StopTimer()
		f.Seek(0, 0) /* seek to beginning of file */
	}
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

func BenchmarkFileWrite_NoRemove(b *testing.B) {
	f := CreateOrDie("write_hello.txt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Write([]byte("Hello World\n"))
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie("write_hello.txt")
}

func BenchmarkFileWrite_RemovePerIter(b *testing.B) {
	f := CreateOrDie("write_hello.txt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		f.Write([]byte("Hello World\n"))
		b.StopTimer()
		f.Close()
		RemoveOrDie("write_hello.txt")
		f = CreateOrDie("write_hello.txt")
	}
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
	offset := int64(5)
	whence := 0 /* offset relative to file origin */
	f := CreateOrDie("hello_world.txt")
	f.Write([]byte("Hello World\n"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Seek(offset, whence)
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie("hello_world.txt")
}

func BenchmarkFilePread64(b *testing.B) {
	offset := int64(0)
	f := CreateOrDie("hello_world.txt")
	f.Write([]byte("Hello World\n"))
	f.Close()
	f = OpenOrDie("hello_world.txt")
	buf := make([]byte, 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.ReadAt(buf, offset)
	}
	b.StopTimer()
	RemoveOrDie("hello_world.txt")
}

func BenchmarkFilePwrite64(b *testing.B) {
	offset := int64(5)
	f := CreateOrDie("hello_world.txt")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.WriteAt([]byte("hello_world\n"), offset)
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie("hello_world.txt")
}

func BenchmarkGetpagesize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Getpagesize()
	}
}

func BenchmarkExecutable(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Executable()
	}
}

func BenchmarkGetpid(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Getpid()
	}
}

func BenchmarkGetppid(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Getppid()
	}
}

func BenchmarkGetwd(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Getwd()
	}
}

func BenchmarkReaddir(b *testing.B) {
	MkdirOrDie("temp")
	CreateOrDie("temp/hello_world.txt")
	f := OpenOrDie("temp")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		f.Readdir(0) /* read all files in directory */
		b.StopTimer()
		f.Seek(0, 0) /* seek to beginning of directory */
	}
	RemoveOrDie("temp")
}

// Helpers to reduce boilerplate code

func MkdirOrDie(name string) {
	err := os.MkdirAll(name, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
}

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
	err := os.RemoveAll(name)
	if err != nil {
		log.Fatal(err)
	}
}
