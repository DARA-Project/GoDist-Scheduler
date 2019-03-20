package main

import (
	"log"
	"net"
	"os"
	"testing"
	"time"
)

const textFile = "hello_world.txt"
const ipport = "127.0.0.1:12345"

func BenchmarkFileOpen(b *testing.B) {
	f := CreateOrDie(textFile)
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.Open(textFile)
	}
	b.StopTimer()
	RemoveOrDie(textFile)

}

func BenchmarkFileRead(b *testing.B) {
	f := CreateOrDie(textFile)
	f.Write([]byte("Hello World\n"))
	f.Close()
	f = OpenOrDie(textFile)
	b1 := make([]byte, 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		f.Read(b1)
		b.StopTimer()
		f.Seek(0, 0) // seek to beginning of file
	}
	f.Close()
	RemoveOrDie(textFile)
}

func BenchmarkFileClose(b *testing.B) {
	f := CreateOrDie(textFile)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Close()
		b.StopTimer()
		f = OpenOrDie(textFile)
		b.StartTimer()
	}
	RemoveOrDie(textFile)
}

func BenchmarkFileWrite_NoRemove(b *testing.B) {
	f := CreateOrDie(textFile)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Write([]byte("Hello World\n"))
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie(textFile)
}

func BenchmarkFileWrite_RemovePerIter(b *testing.B) {
	f := CreateOrDie(textFile)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		f.Write([]byte("Hello World\n"))
		b.StopTimer()
		f.Close()
		RemoveOrDie(textFile)
		f = CreateOrDie(textFile)
	}
	RemoveOrDie(textFile)
}

func BenchmarkFileFstat(b *testing.B) {
	f := CreateOrDie(textFile)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Stat()
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie(textFile)
}

func BenchmarkFileStat(b *testing.B) {
	f := CreateOrDie(textFile)
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.Stat(textFile)
	}
	b.StopTimer()
	RemoveOrDie(textFile)
}

func BenchmarkFileLstat(b *testing.B) {
	f := CreateOrDie(textFile)
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.Lstat(textFile)
	}
	b.StopTimer()
	RemoveOrDie(textFile)
}

func BenchmarkFileLseek(b *testing.B) {
	offset := int64(5)
	whence := 0 // offset relative to file origin
	f := CreateOrDie(textFile)
	f.Write([]byte("Hello World\n"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Seek(offset, whence)
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie(textFile)
}

func BenchmarkFilePread64(b *testing.B) {
	offset := int64(0)
	f := CreateOrDie(textFile)
	f.Write([]byte("Hello World\n"))
	f.Close()
	f = OpenOrDie(textFile)
	buf := make([]byte, 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.ReadAt(buf, offset)
	}
	b.StopTimer()
	RemoveOrDie(textFile)
}

func BenchmarkFilePwrite64(b *testing.B) {
	offset := int64(5)
	f := CreateOrDie(textFile)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.WriteAt([]byte("hello_world\n"), offset)
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie(textFile)
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
		f.Readdir(0) // read all files in directory
		b.StopTimer()
		f.Seek(0, 0) // seek to beginning of directory
	}
	RemoveOrDie("temp")
}

func BenchmarkReaddirnames(b *testing.B) {
	MkdirOrDie("temp")
	CreateOrDie("temp/hello_world.txt")
	f := OpenOrDie("temp")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		f.Readdirnames(0) // read all files in directory
		b.StopTimer()
		f.Seek(0, 0) // seek to beginning of directory
	}
	RemoveOrDie("temp")
}

func BenchmarkWait4(b *testing.B) {
	// Parameters to start a new process that sleeps forever,
	// so we can reliably kill it.
	argc := "/bin/sleep"
	argv := []string{"infinity"}
	attr := &os.ProcAttr{
		Dir:   "",
		Files: nil,
		Env:   nil,
		Sys:   nil,
	}
	b.ResetTimer()
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		proc, err := os.StartProcess(argc, argv, attr)
		if err != nil {
			log.Fatal(err)
		}
		err = proc.Kill()
		if err != nil {
			log.Fatal(err)
		}
		b.StartTimer()
		proc.Wait()
		b.StopTimer()
	}
}

func BenchmarkKill(b *testing.B) {
	// Parameters to start a new process that sleeps forever,
	// so we can reliably kill it.
	argc := "/bin/sleep"
	argv := []string{"infinity"}
	attr := &os.ProcAttr{
		Dir:   "",
		Files: nil,
		Env:   nil,
		Sys:   nil,
	}
	b.ResetTimer()
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		proc, err := os.StartProcess(argc, argv, attr)
		if err != nil {
			log.Fatal(err)
		}
		b.StartTimer()
		proc.Kill()
		b.StopTimer()
		// Wait for process to terminate so we don't use up
		// the allotted number of user processes.
		proc.Wait()
	}
}

func BenchmarkGetuid(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Getuid()
	}
}

func BenchmarkGeteuid(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Geteuid()
	}
}

func BenchmarkGetgid(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Getgid()
	}
}

func BenchmarkGetegid(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Getegid()
	}
}

func BenchmarkGetgroups(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Getgroups()
	}
}

func BenchmarkRename(b *testing.B) {
	f := CreateOrDie(textFile)
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.Rename(textFile, textFile)
	}
	b.StopTimer()
	RemoveOrDie(textFile)
}

func BenchmarkTruncate(b *testing.B) {
	f := CreateOrDie(textFile)
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Truncate file to some arbitrary size.
		os.Truncate(textFile, 12)
	}
	b.StopTimer()
	RemoveOrDie(textFile)
}

func BenchmarkLink(b *testing.B) {
	f := CreateOrDie(textFile)
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		os.Link(textFile, "new.txt")
		b.StopTimer()
		RemoveOrDie("new.txt")
	}
	RemoveOrDie(textFile)
}

func BenchmarkSymlink(b *testing.B) {
	f := CreateOrDie(textFile)
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		os.Symlink(textFile, "new.txt")
		b.StopTimer()
		RemoveOrDie("new.txt")
	}
	RemoveOrDie(textFile)
}

func BenchmarkPipe2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		r, w, err := os.Pipe()
		b.StopTimer()
		if err != nil {
			log.Fatal(err)
		}
		r.Close()
		w.Close()
	}
}

func BenchmarkMkdir(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		os.Mkdir("temp", os.ModePerm)
		b.StopTimer()
		RemoveOrDie("temp")
	}
}

func BenchmarkChdir(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Chdir(".")
	}
}

func BenchmarkUnsetenv(b *testing.B) {
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		err := os.Setenv("new_key", "new_value")
		if err != nil {
			log.Fatal(err)
		}
		b.StartTimer()
		os.Unsetenv("new_key")
		b.StopTimer()
	}
}

func BenchmarkGetenv(b *testing.B) {
	err := os.Setenv("new_key", "new_value")
	if err != nil {
		log.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.Getenv("new_key")
	}
	b.StopTimer()
	err = os.Unsetenv("new_key")
	if err != nil {
		log.Fatal(err)
	}
}

func BenchmarkSetenv(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		os.Setenv("new_key", "new_value")
		b.StopTimer()
		err := os.Unsetenv("new_key")
		if err != nil {
			log.Fatal(err)
		}
	}
}

func BenchmarkClearenv(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Clearenv()
	}
}

func BenchmarkEnviron(b *testing.B) {
	for i := 0; i < b.N; i++ {
		os.Environ()
	}
}

func BenchmarkTimenow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		time.Now()
	}
}

func BenchmarkReadlink(b *testing.B) {
	f := CreateOrDie(textFile)
	f.Close()
	err := os.Symlink(textFile, "link.txt")
	if err != nil {
		log.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.Readlink("link.txt")
	}
	b.StopTimer()
	RemoveOrDie("link.txt")
	RemoveOrDie(textFile)
}

func BenchmarkChmod(b *testing.B) {
	f := CreateOrDie(textFile)
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.Chmod(textFile, os.ModePerm)
	}
	b.StopTimer()
	RemoveOrDie(textFile)
}

func BenchmarkFchmod(b *testing.B) {
	f := CreateOrDie(textFile)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Chmod(os.ModePerm)
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie(textFile)
}

func BenchmarkChown(b *testing.B) {
	uid := os.Getuid()
	gid := os.Getgid()
	f := CreateOrDie(textFile)
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.Chown(textFile, uid, gid)
	}
	b.StopTimer()
	RemoveOrDie(textFile)
}

func BenchmarkLchown(b *testing.B) {
	uid := os.Getuid()
	gid := os.Getgid()
	f := CreateOrDie(textFile)
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.Lchown(textFile, uid, gid)
	}
	b.StopTimer()
	RemoveOrDie(textFile)
}

func BenchmarkFchown(b *testing.B) {
	uid := os.Getuid()
	gid := os.Getgid()
	f := CreateOrDie(textFile)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Chown(uid, gid)
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie(textFile)
}

func BenchmarkFtruncate(b *testing.B) {
	f := CreateOrDie(textFile)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Truncate(10)
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie(textFile)
}

func BenchmarkFsync(b *testing.B) {
	f := CreateOrDie(textFile)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Sync()
	}
	b.StopTimer()
	f.Close()
	RemoveOrDie(textFile)
}

func BenchmarkUtimes(b *testing.B) {
	now := time.Now()
	f := CreateOrDie(textFile)
	f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.Chtimes(textFile, now, now)
	}
	b.StopTimer()
	RemoveOrDie(textFile)
}

func BenchmarkFchdir(b *testing.B) {
	f := CreateOrDie(textFile)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Chdir()
	}
	b.StopTimer()
	RemoveOrDie(textFile)
}

func BenchmarkSetDeadline(b *testing.B) {
	now := time.Now()
	r, w, err := os.Pipe()
	if err != nil {
		log.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.SetDeadline(now)
	}
	b.StopTimer()
	r.Close()
	w.Close()
}

func BenchmarkSetReadDeadline(b *testing.B) {
	now := time.Now()
	r, w, err := os.Pipe()
	if err != nil {
		log.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.SetReadDeadline(now)
	}
	b.StopTimer()
	r.Close()
	w.Close()
}

func BenchmarkSetWriteDeadline(b *testing.B) {
	now := time.Now()
	r, w, err := os.Pipe()
	if err != nil {
		log.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.SetWriteDeadline(now)
	}
	b.StopTimer()
	r.Close()
	w.Close()
}

func BenchmarkNetRead(b *testing.B) {
	serverConn, clientConn := GenerateTCPConnPair()
	buf := make([]byte, 20)
	b.ResetTimer()
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		serverConn.Write([]byte("hello world\n"))
		b.StartTimer()
		clientConn.Read(buf)
		b.StopTimer()
	}
	serverConn.Close()
	clientConn.Close()
}

func BenchmarkNetWrite(b *testing.B) {
	serverConn, clientConn := GenerateTCPConnPair()
	buf := make([]byte, 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		clientConn.Write([]byte("hello world\n"))
		b.StopTimer()
		serverConn.Read(buf)
	}
	serverConn.Close()
	clientConn.Close()
}

func BenchmarkNetClose(b *testing.B) {
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		serverConn, clientConn := GenerateTCPConnPair()
		b.StartTimer()
		clientConn.Close()
		b.StopTimer()
		serverConn.Close()
	}
}

func BenchmarkNetSetDeadline(b *testing.B) {
	t := time.Now()
	serverConn, clientConn := GenerateTCPConnPair()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clientConn.SetDeadline(t)
	}
	b.StopTimer()
	serverConn.Close()
	clientConn.Close()
}

func BenchmarkNetSetReadDeadline(b *testing.B) {
	t := time.Now()
	serverConn, clientConn := GenerateTCPConnPair()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clientConn.SetReadDeadline(t)
	}
	b.StopTimer()
	serverConn.Close()
	clientConn.Close()
}

func BenchmarkNetSetWriteDeadline(b *testing.B) {
	t := time.Now()
	serverConn, clientConn := GenerateTCPConnPair()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clientConn.SetWriteDeadline(t)
	}
	b.StopTimer()
	serverConn.Close()
	clientConn.Close()
}

func BenchmarkNetSetReadBuffer(b *testing.B) {
	serverConn, clientConn := GenerateTCPConnPair()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clientConn.SetReadBuffer(100)
	}
	b.StopTimer()
	serverConn.Close()
	clientConn.Close()
}

func BenchmarkNetSetWriteBuffer(b *testing.B) {
	serverConn, clientConn := GenerateTCPConnPair()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clientConn.SetWriteBuffer(100)
	}
	b.StopTimer()
	serverConn.Close()
	clientConn.Close()
}

func BenchmarkSocket(b *testing.B) {
	addr, err := net.ResolveTCPAddr("tcp", ipport)
	if err != nil {
		log.Fatal(err)
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		clientConn, err := net.DialTCP("tcp", nil, addr)
		b.StopTimer()
		if err != nil {
			log.Fatal(err)
		}
		serverConn, err := listener.AcceptTCP()
		if err != nil {
			log.Fatal(err)
		}
		serverConn.Close()
		clientConn.Close()
	}
	listener.Close()
}

func BenchmarkListenTCP(b *testing.B) {
	addr, err := net.ResolveTCPAddr("tcp", ipport)
	if err != nil {
		log.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StartTimer()
		listener, err := net.ListenTCP("tcp", addr)
		b.StopTimer()
		if err != nil {
			log.Fatal(err)
		}
		listener.Close()
	}

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

func GenerateTCPConnPair() (*net.TCPConn, *net.TCPConn) {
	addr, err := net.ResolveTCPAddr("tcp", ipport)
	if err != nil {
		log.Fatal(err)
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	clientConn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		log.Fatal(err)
	}
	serverConn, err := listener.AcceptTCP()
	if err != nil {
		log.Fatal(err)
	}
	return serverConn, clientConn
}
