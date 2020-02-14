package main

import "net"
import "log"
import "bufio"
import "fmt"

func main() {

  fmt.Println("[SampleServer]Launching server...")

  // listen on all interfaces
  ln, err := net.Listen("tcp", ":18081")
  if err != nil {
    log.Fatal(err)
  }
  defer ln.Close()
  fmt.Println("[SampleServer]Listening now...")
  // accept connection on port
  conn, _ := ln.Accept()
  fmt.Println("[SampleServer]Accepted connection")
  // will listen for message to process ending in newline (\n)
  message, _ := bufio.NewReader(conn).ReadString('\n')
  fmt.Println("[SampleServer]Received Message from client")
  // output message received
  // sample process for string received
  // send new string back to client
  conn.Write([]byte(message + "\n"))
  fmt.Println("[SampleServer]Wrote reply to client", message)
}
