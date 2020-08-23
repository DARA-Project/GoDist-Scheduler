package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"time"
	"runtime"
	logging "github.com/op/go-logging"

	"bitbucket.org/bestchai/dinv/dinvRT"
)

const (
	Ack          = 0xFF
	RequestStick = 1
	ReleaseStick = 2
	ExcuseMe     = 3
	SIZEOFINT    = 4
	n            = 1
	BUFF_SIZE    = 1024
	SLEEP_MAX    = 1e8
)

type Msg struct {
	Type int
}

//global state variables
var (
	Eating         bool
	Thinking       bool
	LeftChopstick  bool
	RightChopstick bool
	Excused        bool
	Chopsticks     int
	Eaten          int

	logger   *logging.Logger
	loglevel = 5
)

//Transition into the eating state
func EatingState() {
	Eating = true
	Thinking = false
	LeftChopstick = true
	RightChopstick = true
	Eaten++
	runtime.DaraLog("EatingState","Eating,Thinking,LeftChopstick,RightChopstick,Eaten",Eating,Thinking,LeftChopstick,RightChopstick,Eaten)
	logger.Noticef("Eaten [%d/%d]", Eaten, n)

}

//transition into the thinking state
func ThinkingState() {
	Eating = false
	Thinking = true
	LeftChopstick = false
	RightChopstick = false
	Chopsticks = 0
	runtime.DaraLog("ThinkingState","Eating,Thinking,LeftChopstick,RightChopstick,Eaten",Eating,Thinking,LeftChopstick,RightChopstick,Eaten)
}

//obtain the left chopstick
func LeftChopstickState() {
	Eating = false
	Thinking = true
	LeftChopstick = true
	Chopsticks++
	runtime.DaraLog("LeftChopstickState","Eating,Thinking,LeftChopstick,RightChopstick,Eaten",Eating,Thinking,LeftChopstick,RightChopstick,Eaten)

}

//obtain the right chopstick
func RightChopstickState() {
	Eating = false
	Thinking = true
	RightChopstick = true
	Chopsticks++
	runtime.DaraLog("RightChopstickState","Eating,Thinking,LeftChopstick,RightChopstick,Eaten",Eating,Thinking,LeftChopstick,RightChopstick,Eaten)
}

//structure defining a philosopher
type Philosopher struct {
	id, neighbourId int
	chopstick       chan bool // the left chopstick // inspired by the wikipedia page, left chopsticks should be used first
	neighbour       *net.UDPConn
}

func makePhilosopher(port, neighbourPort int) *Philosopher {
	logger = SetupLogger(loglevel, "Phil-"+fmt.Sprintf("%d", port))
	logger.Debugf("Setting up listing connection on %d\n", port)
	conn, err := net.ListenPacket("udp", ":"+fmt.Sprintf("%d", port))
	if err != nil {
		log.Fatalf("Unable to set up connection Dying right away: %s", err.Error())
	}

	logger.Debugf("listening on %d\n", port)
	var neighbour *net.UDPConn
	var errDial error
	connected := false
	//Continuously try to connect via udp
	for !connected {

		time.Sleep(time.Millisecond)
		neighbourAddr, errR := net.ResolveUDPAddr("udp4", ":"+fmt.Sprintf("%d", neighbourPort))
		PrintErr(errR)
		listenAddr, errL := net.ResolveUDPAddr("udp4", ":"+fmt.Sprintf("%d", port+1000))
		PrintErr(errL)
		neighbour, errDial = net.DialUDP("udp4", listenAddr, neighbourAddr)
		PrintErr(errDial)
		connected = (errR == nil) && (errL == nil) && (errDial == nil)
	}

	//setup chopstick channel triggered when chopsticks are given or
	//received
	chopstick := make(chan bool, 1)
	chopstick <- true
	logger.Debugf("launching chopstick server\n")

	//sleep while the others start
	time.Sleep(time.Millisecond)

	go ListenForChopsticks(port, chopstick, conn)
	return &Philosopher{port, neighbourPort, chopstick, neighbour}
}

func ListenForChopsticks(port int, chopstick chan bool, conn net.PacketConn) {
	defer logger.Debugf("Chopstick #%d\n is down", port) //attempt to show when the chopsticks are no longer available
	//Incomming request handler
	for true {
		time.Sleep(time.Millisecond)
		req, addr := getRequest(conn)
		go func(request int) {
			switch request {
			case ReleaseStick:

				runtime.DaraLog("ReleaseStick","Eating,Thinking,LeftChopstick,RightChopstick,Eaten",Eating,Thinking,LeftChopstick,RightChopstick,Eaten)
				logger.Debugf("Receiving left stick on %d\n", port)
				chopstick <- true
			case RequestStick:
				runtime.DaraLog("RequestStick","Eating,Thinking,LeftChopstick,RightChopstick,Eaten",Eating,Thinking,LeftChopstick,RightChopstick,Eaten)
				logger.Debugf("Stick request receved on %d\n", port) //TODO this is where I cant just give it away for fairness
				select {
				case <-chopstick:
					runtime.DaraLog("GiveStick","Eating,Thinking,LeftChopstick,RightChopstick,Eaten",Eating,Thinking,LeftChopstick,RightChopstick,Eaten)
					logger.Debugf("Giving stick on %d\n", port)
					resp := dinvRT.PackM(Msg{Type: Ack}, fmt.Sprintf("Giving stick on %d\n", port))
					conn.WriteTo(resp, addr)
				case <-time.After(time.Duration(SLEEP_MAX * 1.3)):
					runtime.DaraLog("TimeoutStick","Eating,Thinking,LeftChopstick,RightChopstick,Eaten",Eating,Thinking,LeftChopstick,RightChopstick,Eaten)
					logger.Criticalf("Exiting timed out wait on %d", port)
				}
			case ExcuseMe:
					runtime.DaraLog("Excuse","Eating,Thinking,LeftChopstick,RightChopstick,Eaten",Eating,Thinking,LeftChopstick,RightChopstick,Eaten)
				if !Excused {
					logger.Debugf("%d has been excused from the table\n", port)
				}
				Excused = true
			}
		}(req)
	}
}

//Read incomming udp messages and return the command code and sender address
func getRequest(conn net.PacketConn) (int, net.Addr) {
	var msg Msg
	buf := make([]byte, BUFF_SIZE)
	_, addr, err := conn.ReadFrom(buf[0:])
	if err != nil {
		log.Fatalf("Unable to read while getting a request Dying for a reason %s", err.Error())
	}

	dinvRT.UnpackM(buf, &msg, "reading request")
	return msg.Type, addr
}

//Transition and print state, then sleep for a random amount of time
func (phil *Philosopher) think() {
	ThinkingState()
	logger.Debugf("%d is thinking.\n", phil.id)
	time.Sleep(time.Duration(rand.Int63n(SLEEP_MAX)))
}

//Eat and then wait for a random amount of time
func (phil *Philosopher) eat() {
	EatingState()
	logger.Debugf("%d is eating.\n", phil.id)
	time.Sleep(time.Duration(rand.Int63n(SLEEP_MAX)))
}

//Attempt to gain a chopstic from a neighbouring philosopher
func (phil *Philosopher) getChopsticks() {
	logger.Debugf("request chopstick %d -> %d\n", phil.id, phil.neighbourId)
	timeout := make(chan bool, 1)
	neighbourChopstick := make(chan bool, 1)
	go func() { time.Sleep(time.Duration(SLEEP_MAX * 2)); timeout <- true }()

	//obtain left chopstick, the one owned by this philosopher
	<-phil.chopstick
	LeftChopstickState()
	//timeout function
	logger.Debugf("%v got his chopstick.\n", phil.id)

	//request chopstick function left should be owned
	go func() {
		time.Sleep(time.Millisecond)
		//Send Request to Neighbour
		var msg Msg
		msg.Type = RequestStick
		buf := make([]byte, BUFF_SIZE)

		req := dinvRT.PackM(msg, "requesting chopsticks")
		conn := phil.neighbour
		conn.Write(req)

		//Read response from Neighbour
		_, err := conn.Read(buf[0:])
		if err != nil {
			log.Println("Is this the one that keeps killing me?")
			//Connection most likely timed out, chopstick unatainable
			log.Println(err.Error())
			return
		}

		dinvRT.UnpackM(buf, &msg, "received a chopstick")
		resp := msg.Type
		if resp == Ack {
			logger.Debugf("Received chopstick %d <- %d\n", phil.id, phil.neighbourId)
			neighbourChopstick <- true
			RightChopstickState()
		}
	}()

	select {
	case <-neighbourChopstick:
		//get ready to eat
		return
	case <-timeout:
		logger.Debugf("Timed out on %d\n", phil.id)
		//return left chopstick and try again
		phil.chopstick <- true
		phil.think()
		phil.getChopsticks()
	}
}

func (phil *Philosopher) returnChopsticks() {
	req := dinvRT.PackM(Msg{Type: ReleaseStick}, "returning stick")

	logger.Debugf("Returning stick %d -> %d\n", phil.id, phil.neighbourId)
	conn := phil.neighbour
	conn.Write(req)
	phil.chopstick <- true
	ThinkingState()
}

func (phil *Philosopher) dine() {
	phil.think()
	phil.getChopsticks()
	phil.eat()
	phil.returnChopsticks()
}

//ask to be excused untill someone says you can
func (phil *Philosopher) leaveTable() {
	for true {
		req := dinvRT.PackM(Msg{Type: ExcuseMe}, "excuse me I want to leave")
		conn := phil.neighbour
		conn.Write(req)

		if Excused == true {
			break
		}
		time.Sleep(time.Duration(SLEEP_MAX))
	}
}

//main should take as an argument the port number of the philosoper
//and that of its neighbour
func main() {
	var (
		myPort, neighbourPort int
	)

	flag.IntVar(&myPort, "mP", 8080, "The port number this host will listen on")
	flag.IntVar(&neighbourPort, "nP", 8081, "The port this host neighbour will listen on")
	flag.Parse()
	log.SetFlags(log.Lshortfile)
	philosopher := makePhilosopher(myPort, neighbourPort)
	for i := 0; i < n; i++ {
		logger.Noticef("Iteration %d\n",i+1)
		time.Sleep(time.Millisecond)
		philosopher.dine()
	}
	logger.Noticef("%v is done dining\n", philosopher.id)
	philosopher.leaveTable()
	logger.Noticef("%d left the table\n", philosopher.id)
}

func PrintErr(err error) {
	if err != nil {
		log.Println("Printing Death Error")
		log.Println(err)
		os.Exit(1)
	}
}

func SetupLogger(loglevel int, name string) *logging.Logger {

	logger := logging.MustGetLogger(name)
	format := logging.MustStringFormatter(
		`%{color}%{time:15:04:05.000} %{shortfile} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
	)
	// For demo purposes, create two backend for os.Stderr.
	backend := logging.NewLogBackend(os.Stderr, "", 0)

	// For messages written to backend2 we want to add some additional
	// information to the output, including the used log level and the name of
	// the function.
	backendFormatter := logging.NewBackendFormatter(backend, format)
	// Only errors and more severe messages should be sent to backend1
	backendlevel := logging.AddModuleLevel(backendFormatter)
	backendlevel.SetLevel(logging.Level(loglevel), "")
	// Set the backends to be used.
	logging.SetBackend(backendlevel)
	return logger
}

