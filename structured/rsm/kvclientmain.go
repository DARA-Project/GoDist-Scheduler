// Simple client to connect to the key-value service and exercise the
// key-value RPC API.
//
// Usage: go run kvclientmain.go [clientid] [clock-update-rate] [ip1:port1] ... [ipN:portN]
//
// - [clientid] : the clientid uint8 to use for this client
//
// - [clock-update-rate] : the rate at which the local lamport clock
//                         is reported to the server through a
//                         clockupdate() call. Number of milliseconds
//                         between clockupdate calls.
//
// - [ip1:port1] : the ip and TCP port of the 1st replica listening for clients
// - ...
// - [ipN:portN] : the ip and TCP port of the Nth replica listening for clients

package main

import (
	"fmt"
	"os"
	"./kvlib"
	"strconv"
)

// Main server loop.
func main() {
	// parse args
	usage := fmt.Sprintf("Usage: %s [clientid] [clock-update-rate] [ip1:port1] ... [ipN:portN]\n", os.Args[0])
	if len(os.Args) < 4 {
		fmt.Printf(usage)
		os.Exit(1)
	}

	clientId, err := strconv.ParseUint(os.Args[1], 10, 8)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	clockRate, err := strconv.ParseUint(os.Args[2], 10, 8)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	addresses := []string{}
	for i := 3; i < len(os.Args); i++ {
		addresses = append(addresses, os.Args[i])
	}

	api, err := kvlib.Init(uint8(clientId), uint8(clockRate), addresses)
	checkError(err)

	err = api.Put("Hello", "World")
	checkError(err)
	val, err := api.Get("Hello")
	checkError(err)
	fmt.Println("Value was ", val)
	err = api.Disconnect()
	checkError(err)
	fmt.Println("\nMission accomplished.")
}

// If error is non-nil, print it out and halt.
func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error ", err.Error())
		os.Exit(1)
	}
}

