package main

import "net"
import "log"
import "bufio"

func main() {

  log.Println("[SampleServer]Launching server...")

  // listen on all interfaces
  ln, err := net.Listen("tcp", ":18081")
  if err != nil {
    log.Fatal(err)
  }
  defer ln.Close()
  log.Println("[SampleServer]Listening now...")
  // accept connection on port
  conn, _ := ln.Accept()
  log.Println("[SampleServer]Accepted connection")
  // will listen for message to process ending in newline (\n)
  message, _ := bufio.NewReader(conn).ReadString('\n')
  log.Println("[SampleServer]Received Message from client")
  // output message received
  // sample process for string received
  // send new string back to client
  conn.Write([]byte(message + "\n"))
  log.Println("[SampleServer]Wrote reply to client", message)
}
