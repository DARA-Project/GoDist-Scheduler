package main

import "net"
import "fmt"
import "bufio"

func main() {
   conn, _ := net.Dial("tcp", "127.0.0.1:18081")
   // read in input from stdin
   text := "Hello World\n"
   // send to socket
   fmt.Println("[SampleClient]Writing to conenction")
   fmt.Fprintf(conn, text)
   // listen for reply
   bufio.NewReader(conn).ReadString('\n')
   fmt.Println("[SampleClient]Received response from server")
}
