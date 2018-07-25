package main

import (
	"dara"
	"encoding/json"
	"net/rpc"
	"flag"
	//"fmt"
	"github.com/DARA-Project/GoDist-Scheduler/common"
	"log"
	//"math/rand"
	"net"
	"os"
	"runtime"
	//"runtime/internal/atomic"
	"sync/atomic"
	"time"
	"unsafe"
	//ls "github.com/DARA-Project/dara/servers/logserver"
	ns "github.com/DARA-Project/dara/servers/nameserver"
	overlord "github.com/DARA-Project/dara/overlord"
)

//Command line arguments
var (
	procs = flag.Int("procs", 1, "The number of processes to model check")
	record = flag.Bool("w", false, "Record an execution")
	replay = flag.Bool("r", false, "Replay an execution")
	explore = flag.Bool("e", false, "Explore a recorded execution")
	manual = flag.Bool("m", false, "Manually step through an execution using a udp connection into the scheduler")
	remoteLogging = flag.Bool("rl", false, "Log program state to remote dara log server")
	projectName = flag.String("project", "","Project name to associate with log server")
)

const(
	//The length of the schedule. When recording the system will
	//execute up to dara.SCHEDLEN. The same is true on replay
	RECORDLEN = 100
)

//*****************************************************************/
// 					END SHARED VARIABLES
//*****************************************************************/


//Global variables specific to shared memory
var (
	//pointer to shared memory, unsafe so that it can be casted
	p unsafe.Pointer
	//err is an integer, because it is set from errors in the runtime
	//where there is no error type set
	err int
	//Procchan is the communication channel laid over shared memory
	procchan *[dara.CHANNELS]dara.DaraProc
	//Used to determine which process (runtime) will be run on the
	//next scheduling decision when in record mode
	LastProc int
)

var (
	l *log.Logger
	//dupl connects with a terminal for manual stepping through an
	//execution
	udpl net.PacketConn
	//overloard server structure
	//buffer for reading from the manual event stepper, the values
	//read into the buffer really don't matter yet
	//TODO allow users to specifcy scheduling decisions based on udp
	//stepping
	udpb []byte
	rpcClient *rpc.Client
)

func checkargs() {
	flag.Parse()
	//Set exactly one arg for record or replay
	if *record && *replay {
		l.Fatal("enable either replay or record, not both")
	}
	if !(*record || *replay || *explore) {
		l.Fatal("enable replay or record or explore")
	}
	if *record {
		l.Print("Recording")
	} else if *explore {
		l.Print("Exploring")
	} else {
		l.Print("Replaying")
	}

	if *remoteLogging {
		if *projectName == "" {
			l.Fatal("projectname=\"\" When logging remotely a project name must be specified")
		}
		names, err := overlord.ResolveServerNames()
		if err != nil {
			l.Fatal(err)
		}
		//just get name server
		var logserverName ns.Server
		for _, s := range names.Servers {
			if s.Type == ns.LOG {
				logserverName = s
			}
		}
		if logserverName.Name == "" {
			//no remote log server found
			//TODO look for one locally
			l.Fatal("Unable to resolve a remote logserver")
		}
		var servers overlord.ServerPool
		servers[ns.LOG] = make(map[string]overlord.ServerInstance,0)
		overlord.DialServers([]ns.Server{logserverName},&servers)
		if len(servers) < 1 {
			l.Fatal("Unable to connect to log server")
		}
		for _, instance := range servers[ns.LOG] {
			rpcClient = instance.RPC
			break // quit after just one, this loop is just inconvienient
		}
		l.Println(names)
		//Use the first remote logging server available

		//connect to name server
		//get log server
	}
	return
}

func forward() {
	if *manual {
		udpl.ReadFrom(udpb)
		l.Print(udpb)
	} else {
		time.Sleep(1 *time.Millisecond)
	}
}

	//global schedule
	//TODO make all schedule operations object calls on the scedule
	//(ie schedule.next())
var schedule dara.Schedule

func roundRobin() int {
	LastProc = (LastProc + 1) % (*procs+1)
	if LastProc == 0 {
		LastProc++
	}
	return LastProc
}

func ConsumeAndPrint(ProcID int) []dara.Event{
	cl := ConsumeLog(ProcID)

	for i := range cl {
		l.Println(cl[i])
		//for j := range cl[i].Vars {
		//	l.Println(cl[i].Vars[j])
		//}
	}
	return cl
}

func ConsumeLog(ProcID int) []dara.Event {
	logsize := procchan[ProcID].LogIndex
	log := make([]dara.Event,logsize)
	for i:=0;i<logsize;i++ {
		ee := &(procchan[ProcID].Log[i])
		e := &(log[i])
		(*e).Type = (*ee).Type
		(*e).P = (*ee).P
		(*e).G = (*ee).G
		(*e).Epoch = (*ee).Epoch
		(*e).LE.LogID = string((*ee).ELE.LogID[:])
		(*e).LE.Vars = make([]dara.NameValuePair,(*ee).ELE.Length)
		l.Printf("Reading Logging Instance P=%d G=%s ID=%s Length=%d",(*e).P,common.RoutineInfoString(&((*e).G)),(*e).LE.LogID,len((*e).LE.Vars))
		for j:=0;j<len((*e).LE.Vars);j++{
			(*e).LE.Vars[j].VarName = string((*ee).ELE.Vars[j].VarName[:])
			(*e).LE.Vars[j].Value = runtime.DecodeValue((*ee).ELE.Vars[j].Type,(*ee).ELE.Vars[j].Value)
			(*e).LE.Vars[j].Type = string((*ee).ELE.Vars[j].Type[:])
		}
		(*e).SyscallInfo = (*ee).SyscallInfo
		//(*e).Msg = (*ee).EM //TODO complete messages
	}
	procchan[ProcID].LogIndex = 0
	procchan[ProcID].Epoch++
	return log
}

func replay_sched() {
	var i int
	f, err := os.Open("Schedule.json")
	if err != nil {
		l.Fatal(err)
	}
	dec := json.NewDecoder(f)
	err = dec.Decode(&schedule)
	if err != nil {
		l.Fatal(err)
	}
	for i<len(schedule) {
		if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].P].Lock))),dara.UNLOCKED,dara.LOCKED) {
			if procchan[schedule[i].P].Run == -1 { //TODO check predicates on goroutines + schedule

				//move forward
				forward()

				l.Print("Scheduling Event")
				//TODO send a proper signal to the runtime with
				//the goroutine ID that needs to be run
				runnable := make([]int,0)
				for j, info := range procchan[schedule[i].P].Routines {
					if dara.GetDaraProcStatus(info.Status) != dara.Idle {
						l.Printf("Proc[%d]Routine[%d].Info = %s",schedule[i].P,j,common.RoutineInfoString(&info))
						runnable = append(runnable,j)
					}
				}
				//TODO clean up init
				runningindex := 0
				if len(runnable) == 0 {
					//This should only occur when the processes
					//have not started yet. There should be a
					//smarter starting condition here rather than
					//just borking on the init of the schedule
					runningindex = -2
				} else {
					//replay schedule
					runningindex = schedule[i].G.Gid
				}
				//Assign which goroutine to run
				procchan[schedule[i].P].Run = runningindex //TODO make this the scheduled GID
				procchan[schedule[i].P].RunningRoutine = schedule[i].G //TODO make this the scheduled GID
				l.Printf("Running (%d/%d) %d %s",i+1,len(schedule),schedule[i].P,common.RoutineInfoString(&schedule[i].G))

				//TODO explore schedule if in explore mode?
				atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].P].Lock))),dara.UNLOCKED)
				//l.Print(procchan)
				for {
					if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].P].Lock))),dara.UNLOCKED,dara.LOCKED) {
						if procchan[schedule[i].P].Run == -1 {
							atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].P].Lock))),dara.UNLOCKED)
							i++
							for schedule[i].Type != dara.SCHED_EVENT {
								i++
							}
							break
						}
						atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].P].Lock))),dara.UNLOCKED)
						//TODO log.Fatalf("Preemtion is turned on")
					}
				}
				time.Sleep(time.Second)
			}
		}
	}
	//End computation
	l.Printf("Wake me up when september ends")
	time.Sleep(time.Second)
	l.Printf("1st october")
	for i := 1;i <= *procs;i++{
		for {
			if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[i].Lock))),dara.UNLOCKED,dara.LOCKED) {
				l.Printf("Stopping Proc %d",i)
				procchan[i].Run = -4
				atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[i].Lock))),dara.UNLOCKED)
				break
			}
		}
	}
	l.Println("Replay is over")
}


func record_sched() {
	LastProc = -1
	ProcID := roundRobin()
	var i int
	for i<RECORDLEN {
		//else busy wait
		if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED,dara.LOCKED) {
			if procchan[ProcID].Run == -1 { //TODO check predicates on goroutines + schedule
				forward()
				procchan[ProcID].Run = -3

				atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED)
				flag := true
				for ; flag; {
					if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED,dara.LOCKED) {
						//l.Printf("procchan[schedule[%d]].Run = %d",i,procchan[schedule[i]].Run)
						if procchan[ProcID].Run != -3 {
							l.Printf("Recording Event on %d\n",ProcID)
							//Update the last running routine
							l.Printf("Recording Event %d",i)
							events := ConsumeAndPrint(ProcID)
							schedule = append(schedule,events...)
							//Set the status of the routine that just
							//ran
							if (i >= RECORDLEN - *procs) {
								//mark the processes for death
								l.Printf("Ending Execution!")
								procchan[ProcID].Run = -4 //mark process for death
							} else {
								procchan[ProcID].Run = -1	//lock process
							}

							ProcID = roundRobin()
							i++
							flag = false
						}
						//l.Print("Still running")
						atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED)
					}
					time.Sleep(time.Microsecond)
				}
			}
			f, erros := os.Create("Schedule.json")
			if erros != nil {
				l.Fatal(err)
			}
			enc := json.NewEncoder(f)
			enc.Encode(schedule)
		}
	}
	l.Printf(common.ScheduleString(&schedule))
	l.Printf("%+v\n",schedule)
	l.Println("The End")
}

func explore_sched() {
	l.Println("Placeholder for exploring")
}

func main() {
	//Set up logger
	l = log.New(os.Stdout,"[Scheduler]",log.Lshortfile)
	checkargs()
	l.Println("Starting the Scheduler")


	//If manual step forward in time by reading udp packets
	if *manual {
		var errudp error
		udpl, errudp = net.ListenPacket("udp", "localhost:6666")
		udpb = make([]byte,128)
		if errudp !=nil {
			l.Fatal(errudp)
		}
	}


	//Init MMAP (this should be moved to the init function
	p , err = runtime.Mmap(nil,dara.SHAREDMEMPAGES*dara.PAGESIZE,dara.PROT_READ|dara.PROT_WRITE ,dara.MAP_SHARED,dara.DARAFD,0)

	if err != 0 {
		l.Fatal(err)
	}

	//TODO can probably be deleted
	time.Sleep(time.Second * 1)

	//Map control struct into shared memory
	procchan = (*[dara.CHANNELS]dara.DaraProc)(p)
	//rand.Seed(int64(time.Now().Nanosecond()))
	//var count int
	for i:=range procchan {
		l.Printf("Unlocking %d",i)
		procchan[i].Lock = dara.UNLOCKED
		procchan[i].SyscallLock = dara.UNLOCKED
		procchan[i].Run = -1
		procchan[i].Syscall = -1
	}
	//State = RECORD

	if *replay {
		replay_sched()
		l.Println("Finished replaying")
	} else if *record {
		record_sched()
		l.Println("Finished recording")
	} else if *explore {
		explore_sched()
		l.Println("Finished exploring")
	}
	l.Println("Backstreet's back again")
}


//Printing Functions
