//This is a user interface that sends messages to the scheduler
package main

import (
	"log"
	"net"
	"bufio"
	"os"
)


func main() {
	//controll the execution of threads manually
	reader := bufio.NewReader(os.Stdin)
	conn, err := net.Dial("udp", ":6666")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	for {
		//manual control
		input, _ := reader.ReadString('\n')
		conn.Write([]byte(input))
	}
}





