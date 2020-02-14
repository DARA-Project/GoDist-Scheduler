package main

import "net"
import "fmt"
import "bufio"
import "log"

func main() {
    fmt.Println("[SampleClient] Starting the client")
    conn, err := net.Dial("tcp", "127.0.0.1:18081")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("[SampleClient] Dialed the server successfully")
    // read in input from stdin
    text := "Hello World\n"
    // send to socket
    fmt.Println("[SampleClient]Writing to conenction")
    fmt.Fprintf(conn, text)
    // listen for reply
    bufio.NewReader(conn).ReadString('\n')
    fmt.Println("[SampleClient]Received response from server")
}
