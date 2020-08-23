// A simple key-value store that supports four API calls over RPC:
// val <- get(key,clientid,clock): execute k-v get from clientid with lamport clock value
// - put(key,val,clientid,clock) : execute k-v put from clientid with lamport clock value
// - clockupdate(clientid,clock) : report latest lamport clock value at clientid
// - disconnect(clientid)        : report disconnection/failure of clientid
//
// Usage: go run kvservicemain.go [ip:port] [num-clients]
//
// - [ip:port] : the ip and TCP port on which the service will listen
//               for connections
//
// - [num-clients] : Number of unique clients to expect (unique clientid's)

package main

import (
	"fmt"
	"log"
//	"math/rand"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
)

// args in get(args)
type GetArgs struct {
	Key      string // key to look up
	Clientid uint8  // client id issuing this get
	Clock    uint64 // value of lamport clock at the issuing client
}

// args in put(args)
type PutArgs struct {
	Key      string // key to associate value with
	Val      string // value
	Clientid uint8  // client id issueing this put
	Clock    uint64 // value of lamport clock at the issuing client
}

// args in clockupdate(args)
type ClockUpdateArgs struct {
	Clientid uint8  // client id issueing this put
	Clock    uint64 // value of lamport clock at the issuing client
}

// args in disconnect(args)
type DisconnectArgs struct {
	Clientid uint8 // client id issueing this put
}

// Reply from service for all the API calls above.
type ValReply struct {
	Val string // value; depends on the call
}

// Value in the key-val store.
type MapVal struct {
	value string // the underlying value representation
}

type ClientInfo struct {
	Clock uint64
	Alive bool
	LastReq uint64
}

// Map implementing the key-value store: use this for k-v storage.
var kvmap map[string]*MapVal
var kvMux sync.Mutex

var clientinfomap map[uint8]*ClientInfo
var cliMux sync.Mutex

type KeyValService int

// Command line arg.
var numClients uint8

func updateAddClientClock(clientID uint8, clock uint64) {
	if client, ok := clientinfomap[clientID]; ok {
		client.Clock = clock
		clientinfomap[clientID] = client
	} else {
		client := &ClientInfo{clock, true, 0}
		clientinfomap[clientID] = client
	}
}

func addClientRequestInfo(clientID uint8, request uint64) {
	if client, ok := clientinfomap[clientID]; ok {
		client.LastReq = request
		clientinfomap[clientID] = client
	}
}

func deleteClientRequestInfo(clientID uint8) {
	if client, ok := clientinfomap[clientID]; ok {
		client.LastReq = 0
		clientinfomap[clientID] = client
	}
}

func check_stability(clientID uint8, clock uint64) bool {
	var clients_checked uint8
	clients_checked = 0
	for cid, client := range clientinfomap {
		// Don't check for dead clients for stability
		if client.Alive == false {
			clients_checked += 1
			continue
		}
		if clientID == cid {
			clients_checked += 1
			continue
		}
		// Check if the client has a lower or equal clock so we must return as this request is not stable yet.
		if client.Clock <= clock {
			return false
		}
		// Check if any client has an earlier pending request which should be served first
		if client.LastReq != 0 && client.LastReq < clock {
			return false
		}
		// Tie-breaker rule
		if client.LastReq == clock && cid < clientID {
			return false
		}
		clients_checked += 1
	}
	// Message is not stable if we have not received a single other message from any other client.
	if clients_checked != numClients { 
		return false
	}
	return true
}

func returnWhenStable(clientID uint8, clock uint64) {
	is_stable := false
	for {
		if is_stable {
			break
		}
		ticker := time.NewTicker(100 * time.Millisecond)
		select {
			case <- ticker.C:
				cliMux.Lock()
				is_stable = check_stability(clientID, clock)
				cliMux.Unlock()
		}
	}
}

// GET
func (kvs *KeyValService) Get(args *GetArgs, reply *ValReply) error {
	clientID := args.Clientid
	clock := args.Clock
	log.Printf("[GET-R] Client %d Clock %d\n", clientID, clock)
	cliMux.Lock()
	updateAddClientClock(clientID, clock)
	addClientRequestInfo(clientID, clock)
	cliMux.Unlock()
	returnWhenStable(clientID, clock)
	kvMux.Lock()
	key := args.Key
	if v, ok := kvmap[key]; ok {
		reply.Val = v.value
	} else {
		reply.Val = ""
	}
	kvMux.Unlock()
	cliMux.Lock()
	deleteClientRequestInfo(clientID)
	cliMux.Unlock()
	log.Printf("[GET-S] Client %d Clock %d\n", clientID, clock)
	return nil
}

// PUT
func (kvs *KeyValService) Put(args *PutArgs, reply *ValReply) error {
	clientID := args.Clientid
	clock := args.Clock
	log.Printf("[PUT-R] Client %d Clock %d\n", clientID, clock)
	cliMux.Lock()
	updateAddClientClock(clientID, clock)
	addClientRequestInfo(clientID, clock)
	cliMux.Unlock()
	returnWhenStable(clientID, clock)
	kvMux.Lock()
	key := args.Key
	val := args.Val
	mapVal := &MapVal{val}
	kvmap[key] = mapVal
	reply.Val = ""
	kvMux.Unlock()
	cliMux.Lock()
	deleteClientRequestInfo(clientID)
	cliMux.Unlock()
	log.Printf("[PUT-S] Client %d Clock %d\n", clientID, clock)
	return nil
}

// CLOCKUPDATE
func (kvs *KeyValService) ClockUpdate(args *ClockUpdateArgs, reply *ValReply) error {
	clientID := args.Clientid
	clock := args.Clock
	//log.Printf("[UPD-R] Client %d Clock %d\n", clientID, clock)
	cliMux.Lock()
	updateAddClientClock(clientID, clock)
	cliMux.Unlock()
	//log.Printf("[UPD-S] Client %d Clock %d\n", clientID, clock)
	return nil
}

// DISCONNECT
func (kvs *KeyValService) Disconnect(args *DisconnectArgs, reply *ValReply) error {
	clientID := args.Clientid
	log.Printf("[DSN-R] Client %d\n", clientID)
	cliMux.Lock()
	if client, ok := clientinfomap[clientID]; ok {
		client.Alive = false
		clientinfomap[clientID] = client
	}
	reply.Val = ""
	cliMux.Unlock()
	log.Printf("[DSN-S] Client %d\n", clientID)
	return nil
}

// Main server loop.
func main() {
	// Parse args.
	usage := fmt.Sprintf("Usage: %s [ip:port] [num-clients]\n", os.Args[0])
	if len(os.Args) != 3 {
		fmt.Printf(usage)
		os.Exit(1)
	}

	ip_port := os.Args[1]
	arg, err := strconv.ParseUint(os.Args[2], 10, 8)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if arg == 0 {
		fmt.Printf(usage)
		fmt.Printf("\tnum-clients arg must be non-zero\n")
		os.Exit(1)
	}
	numClients = uint8(arg)

	// Setup key-value store and register service.
	kvservice := new(KeyValService)
	rpc.Register(kvservice)
	l, e := net.Listen("tcp", ip_port)
	if e != nil {
		log.Fatal("listen error:", e)
	}

	// Init maps
	kvmap = map[string]*MapVal{}
	clientinfomap = map[uint8]*ClientInfo{}

	// TODO: Enter servicing loop, like:

	// for {
	// conn, _ := l.Accept()
	// go rpc.ServeConn(conn)
	// }
	log.Println("Business is open")
	for {
		conn, _ := l.Accept()
		go rpc.ServeConn(conn)
	}
}

