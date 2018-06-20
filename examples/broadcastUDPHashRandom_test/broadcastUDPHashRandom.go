package main

/* This example program tests Dara's ability to reliably replay a
* system with mulitple nodes communicating via UDP. The number of
* nodes in the system is set by the enviornment variable
* DARATESETPEERS. After beginning each peer resolves the UDP addresses
* and saves an MD5 Hash of its ID.

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
)

var (
	logger *log.Logger
	DaraPID int
	DaraTestPeers int
	conn *net.UDPConn
	hash string
	r *rand.Rand
)

func main() {
	logger = log.New(os.Stdout, "[INITALIZING]",log.Lshortfile)
	ParseEnviornment()
	SetupUDPNetworkConnections()
	defer conn.Close()
	logger.SetPrefix(fmt.Sprintf("[Peer %d] ",DaraPID))
	logger.Printf("DaraPID: %d\tDaraTestPeers:%d\n",DaraPID,DaraTestPeers)


	rand.Seed(int64(time.Now().Nanosecond()))
	r = rand.New(rand.NewSource(int64(time.Now().Nanosecond())))

	hashf := md5.New()
	hash = string(hashf.Sum([]byte(fmt.Sprintf("%d",DaraPID))))
	logger.Printf("Hash:%x\n",hash)
	
	time.Sleep(time.Second)
	//Write
	for i:= 0;i<BROADCASTS;i++ {
		broadcast(hash)
		newhash := readhashmsg()
		hash = string(hashf.Sum([]byte(hash+newhash)))
	}
}

func broadcast(h string) {
	peerstart := (r.Int() % (DaraTestPeers+1))
	if peerstart == 0 {
		peerstart = 1
	}
	var counter = 1
	for i:=peerstart;counter<=DaraTestPeers;counter++{
		logger.Printf("BROADCASTING %d Peerstart %d\n",i,peerstart)
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

func readhashmsg() string {
	buf := make([]byte,BUFSIZE)
	n, addr, err := conn.ReadFromUDP(buf)
	if err != nil {
		logger.Panicf("Error reading from udp %s",err.Error())
	}
	logger.Printf("Received: %x From %s Len %d",buf[:n],addr.String(),n)
	return string(buf)
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




