
package main

/* This example program tests Dara's ability to reliably replay a
* system with mulitple nodes communicating via UDP. The number of
* nodes in the system is set by the enviornment variable
* DARATESETPEERS. After beginning each peer resolves the UDP addresses
* and saves an MD5 Hash of its ID. This program is an exstention of
* the other broadcast protocol in that it has multiple threads running
* three seperate tasks. The tasks are 1) reading from the network 2)
* computing the hash, 3) broadcasting hashes. Each task has
* THREADSPERJOB instances, each of which communicates with eachother
* over shared channels in a pipelined fashion picutred below

where THREADSPERJOB = 3

readhashmsg(0)-\          /> calculatehash(0)-\           /> broadcast(0)
readhashmsg(1)--> recchan -> calculatehash(1)--> writechan-> broadcast(1)
readhashmsg(2)-/          \> calculatehash(2)-/           \> broadcast(2)


The algorithm for this program is thus.

1) Each node broadcasts its current MD5 hash starting at a RANDOM peer address
2) Each node waits to recieve a single hash
3) Upon recipt a nodes current hash = M55( hash + receivedHash)
4) Print the current hash + size

This algorithm has the advantage that any nondeterminism in the nodes
will cause the hashes that they compute to differ immidiatly, thereby
making the ouput of the program sensitive to every nondeterminisic
action.

This program is nondermanistic in the following ways

1) The order in which messages are placed on the network by any node.
2) The order in which messagesa are received from the network by all nodes.
3) The order in which each node sends it's messages
4) The order in which nodes process their messages. This refers to the 
	global order of all events.
5) The threadID of each processing node in the pipeline

*/

import (
	"fmt"
	"os"
	"net"
	"log"
	"strconv"
	"crypto/md5"
	"time"
	"math/rand"
)

const (
	BROADCASTS = 50
	BUFSIZE = md5.Size
	THREADSPERJOB = 3
)

var (
	logger *log.Logger
	DaraPID int
	DaraTestPeers int
	conn *net.UDPConn
	hash string
	r *rand.Rand
	recchan chan string
	writechan chan string
	done chan bool
)

func main() {
	logger = log.New(os.Stdout, "[INITALIZING]",log.Lshortfile)
	ParseEnviornment()
	SetupUDPNetworkConnections()
	defer conn.Close()
	logger.SetPrefix(fmt.Sprintf("[Peer %d] ",DaraPID))
	logger.Printf("DaraPID: %d\tDaraTestPeers:%d\n",DaraPID,DaraTestPeers)

	recchan = make(chan string, THREADSPERJOB)
	writechan = make(chan string, THREADSPERJOB)
	done = make(chan bool,1)

	rand.Seed(int64(time.Now().Nanosecond()))
	r = rand.New(rand.NewSource(int64(time.Now().Nanosecond())))

	hashf := md5.New()
	hash = string(hashf.Sum([]byte(fmt.Sprintf("%d",DaraPID))))
	logger.Printf("Hash:%x\n",hash)
	
	time.Sleep(time.Second)

	//Startup Threads
	for i:= 0;i<THREADSPERJOB;i++ {
		logger.Printf("Starting job set %d\n",i)
		go readhashmsg(i)
		go calculateHash(i)
		go broadcast(i)
	}

	writechan <- hash
	<-done
	logger.Printf("EXIT")



}

func calculateHash(tid int) {
	logger.Printf("Calculating Hashes on %d",tid)
	hashf := md5.New()
	for {
		rechash := <- recchan
		hash = string(hashf.Sum([]byte(hash+rechash)))
		writechan <- hash
	}
}


func broadcast(tid int) {
	logger.Printf("Broadcasting Hashes on %d",tid)
	for bcasts:=0;bcasts<BROADCASTS;bcasts++{
		h := <- writechan
		peerstart := (r.Int() % (DaraTestPeers+1))
		if peerstart == 0 {
			peerstart = 1
		}
		var counter = 1
		for i:=peerstart;counter<=DaraTestPeers;counter++{
			logger.Printf("BROADCASTING %d Peerstart %d Tid %d Hash% s\n",i,peerstart,tid,h)
			if i == DaraPID {
				i = (i + 1) % (DaraTestPeers+1)
				if i == 0 {
					i=1
				}
				continue
			} else {
				peerAddrString := fmt.Sprintf(":666%d",i)
				peerAddr, err := net.ResolveUDPAddr("udp",peerAddrString)
				if err != nil {
					logger.Panicf("Unable to resolve peer %s: %s",peerAddrString,err)
				}
				n, err := conn.WriteToUDP([]byte(h),peerAddr)
				if err != nil {
					logger.Panicf("Unable to write msg to peer %s",peerAddr.String())
				}
				logger.Printf("Writing: %x\t To: %s\t Len: %d\t",h,peerAddr.String(),n)
				i = (i + 1) % (DaraTestPeers+1)
				if i == 0 {
					i=1
				}
			}
			time.Sleep(time.Millisecond)
		}
	}
	done<-true
}

func readhashmsg(tid int) {
	logger.Printf("Reading Hashes on %d",tid)
	buf := make([]byte,BUFSIZE)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			logger.Panicf("Error reading from udp %s",err.Error())
		}
		logger.Printf("Received: %x From %s Len %d",buf[:n],addr.String(),n)
		recchan <- string(buf[:n])
	}
}

func ParseEnviornment() {
	var err error
	DaraPIDString := os.Getenv("DARAPID")
	if DaraPIDString == "" {
		logger.Fatalf("DARAPID not set!")
	}
	DaraPID, err = strconv.Atoi(DaraPIDString)
	if err != nil {
		logger.Fatalf("DARAPID not a valid integer %s: %s",DaraPIDString,err.Error())
	}

	DaraTESTPEERSString := os.Getenv("DARATESTPEERS")
	if DaraTESTPEERSString == "" {
		logger.Fatalf("DARATESTPEERS not set!")
	}
	DaraTestPeers, err = strconv.Atoi(DaraTESTPEERSString)
	if err != nil {
		logger.Fatalf("DARATESTPEERS not a valid integer %s: %s",DaraTESTPEERSString,err.Error())
	}
	logger.Println("Done Parsing Enviornment")
	return
}

func SetupUDPNetworkConnections() {
	addrstring := fmt.Sprintf(":666%d",DaraPID)
	addr, err := net.ResolveUDPAddr("udp",addrstring)
	if err != nil {
		logger.Fatal(err)
	}
	conn, err = net.ListenUDP("udp",addr)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Println("Done Setting Up Network Connections")
}




