package main

import (
	"dara"
	"encoding/json"
	"net/rpc"
	"flag"
	//"fmt"
	"github.com/DARA-Project/GoDist-Scheduler/common"
	"github.com/DARA-Project/GoDist-Scheduler/explorer"
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
	//ns "github.com/DARA-Project/dara/servers/nameserver"
	//overlord "github.com/DARA-Project/dara/overlord"
)

//Command line arguments
var (
	procs = flag.Int("procs", 1, "The number of processes to model check")
	record = flag.Bool("record", false, "Record an execution")
	replay = flag.Bool("replay", false, "Replay an execution")
	explore = flag.Bool("explore", false, "Explore a recorded execution")
	manual = flag.Bool("m", false, "Manually step through an execution using a udp connection into the scheduler")
	remoteLogging = flag.Bool("rl", false, "Log program state to remote dara log server")
	projectName = flag.String("project", "","Project name to associate with log server")
)

const(
	//The length of the schedule. When recording the system will
	//execute up to dara.SCHEDLEN. The same is true on replay
	RECORDLEN = 10000
	EXPLORELEN = 100
	EXPLORATION_LOG_FILE = "visitedLog.txt"
	MAX_EXPLORATION_DEPTH = 10
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

	/*
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
	*/
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

	return cl
}

func ConsumeLog(ProcID int) []dara.Event {
    // The 2 print statements somehow have different values for SimpleFileRead. How the fuck.....
    l.Println("Current log event index is", procchan[ProcID].LogIndex)
	logsize := procchan[ProcID].LogIndex
    l.Println("Current log event index is", logsize)
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
		for j:=0;j<len((*e).LE.Vars);j++{
			(*e).LE.Vars[j].VarName = string((*ee).ELE.Vars[j].VarName[:])
			(*e).LE.Vars[j].Value = runtime.DecodeValue((*ee).ELE.Vars[j].Type,(*ee).ELE.Vars[j].Value)
			(*e).LE.Vars[j].Type = string((*ee).ELE.Vars[j].Type[:])
		}
		(*e).SyscallInfo = (*ee).SyscallInfo
		//(*e).Msg = (*ee).EM //TODO complete messages
        l.Println(common.ConciseEventString(e))
	}
	procchan[ProcID].LogIndex = 0
	procchan[ProcID].Epoch++
	return log
}

func CheckAllGoRoutinesDead(ProcID int) bool {
    allDead := true
    l.Printf("Checking if all goroutines are dead yet for Process %d", ProcID)
    //l.Printf("Num routines are %d", len(procchan[ProcID].Routines))
    for _, routine := range procchan[ProcID].Routines {
        // Only check for dead goroutines for real go routines
        if routine.Gid != 0 {
            status := dara.GetDaraProcStatus(routine.Status)
            l.Printf("Status of GoRoutine %d is %s\n", routine.Gid, dara.GStatusStrings[status])
            allDead = allDead && (status == dara.Dead)
        }
    }
    return allDead
}

func GetAllRunnableRoutines(event dara.Event) []int {
	runnable := make([]int,0)
	for j, info := range procchan[event.P].Routines {
		if dara.GetDaraProcStatus(info.Status) != dara.Idle {
			l.Printf("Proc[%d]Routine[%d].Info = %s",event.P,j,common.RoutineInfoString(&info))
			runnable = append(runnable,j)
		}
	}
    return runnable
}

func Is_event_replayable(runnable []int, Gid int) bool {
    for _, id := range runnable {
        if Gid == id {
            return true
        }
    }
    return false
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
	l.Println("Num Events : ", len(schedule))
    l.Println("Replaying the following schedule : ", common.ConciseScheduleString(&schedule))
	for i<len(schedule) {
		if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].P].Lock))),dara.UNLOCKED,dara.LOCKED) {
			if procchan[schedule[i].P].Run == -1 { //check predicates on goroutines + schedule

				//move forward
				forward()

				l.Print("Scheduling Event")
                runnable := GetAllRunnableRoutines(schedule[i])
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
                    can_run := Is_event_replayable(runnable, schedule[i].G.Gid)
                    if !can_run {
                        l.Printf("Can't run event %d\n", i)
                    }
					runningindex = schedule[i].G.Gid
				}
				//Assign which goroutine to run
				procchan[schedule[i].P].Run = runningindex
				procchan[schedule[i].P].RunningRoutine = schedule[i].G
                l.Println("Replaying Event :", common.ConciseEventString(&schedule[i]))
				l.Printf("Running (%d/%d) %d %s",i+1,len(schedule),schedule[i].P,common.RoutineInfoString(&schedule[i].G))


				atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].P].Lock))),dara.UNLOCKED)
				for {
					if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].P].Lock))),dara.UNLOCKED,dara.LOCKED) {
                        currentDaraProc := schedule[i].P
						if procchan[schedule[i].P].Run == -1 {
							atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].P].Lock))),dara.UNLOCKED)
                            events := ConsumeAndPrint(currentDaraProc)
                            l.Println("Replay Consumed ", len(events), "events")
							i += len(events)
                            l.Println("Replay : At event", i)
                            if i >= len(schedule) {
                                l.Printf("Informing the local scheduler end of replay")
                                procchan[currentDaraProc].Run = -4
                                break
                            }
							break
						} else if procchan[schedule[i].P].Run == -100 {
                            // This means that the local runtime finished and that we can move on
                            events := ConsumeAndPrint(currentDaraProc)
                            l.Println("Replay Consumed", len(events), "events")
                            i += len(events)
                            break
                        }
						atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[currentDaraProc].Lock))),dara.UNLOCKED)
					}
				}
				time.Sleep(time.Second)
                //l.Printf("End is nigh")
                //if i >= len(schedule) - 1 {
                //    procchan[schedule[i].P].Run = -4
                //}
			}
		}
	}
	//End computation
	l.Printf("Wake me up when september ends")
	l.Printf("1st october")
    // All Procs should have stopped by now. Don't need to do this.
	//for i := 1;i <= *procs;i++{
	//	for {
	//		if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[i].Lock))),dara.UNLOCKED,dara.LOCKED) {
	//			l.Printf("Stopping Proc %d",i)
	//			procchan[i].Run = -4
	//			atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[i].Lock))),dara.UNLOCKED)
	//			break
	//		}
	//	}
	//}
	l.Println("Replay is over")
}


func record_sched() {
	LastProc = -1
	ProcID := roundRobin()
	var i int
	for i<RECORDLEN {
		//else busy wait
        //l.Printf("Procchan Run status is %d\n", procchan[ProcID].Run)
		if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED,dara.LOCKED) {
            l.Println("Obtained Lock")
            if procchan[ProcID].Run == -100 {
			    events := ConsumeAndPrint(ProcID)
			    schedule = append(schedule,events...)
                break
            }
			if procchan[ProcID].Run == -1 { //TODO check predicates on goroutines + schedule
				forward()
				procchan[ProcID].Run = -3

				atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED)
				flag := true
				for ; flag; {
//                    l.Printf("Stuck in inner loop")
					if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED,dara.LOCKED) {
                        if procchan[ProcID].Run == -100 {
                            l.Printf("Ending discovered")
                            events := ConsumeAndPrint(ProcID)
                            schedule = append(schedule,events...)
                            flag = false
                            continue
                        }
						if procchan[ProcID].Run != -3 {
                            l.Printf("Procchan Run status inside is %d\n", procchan[ProcID].Run)
							l.Printf("Recording Event on Process/Node %d\n",ProcID)
							//Update the last running routine
							l.Printf("Recording Event Number %d",i)
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
                            l.Printf("Found %d log events", len(events))
							i += len(events)
							flag = false
						}
						if procchan[ProcID].LogIndex > 0 {
							l.Printf("Procchan %d\n", procchan[ProcID].Run)
							events := ConsumeAndPrint(ProcID)
							schedule = append(schedule,events...)
							f, erros := os.Create("Schedule.json")
							if erros != nil {
								l.Fatal(err)
							}
							enc := json.NewEncoder(f)
							enc.Encode(schedule)
						}
						//l.Print("Still running")
						//l.Printf("Status is %d\n",procchan[ProcID].RunningRoutine.Status)
						//l.Printf("GoID is %d\n", procchan[ProcID].RunningRoutine.Gid)
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
            if CheckAllGoRoutinesDead(ProcID) {
                break
            }
		}
	}
	//l.Printf(common.ScheduleString(&schedule))
	//l.Printf("%+v\n",schedule)
    l.Println("Recorded schedule is as follows : \n",common.ConciseScheduleString(&schedule))
	l.Println("The End")
}

func explore_init() *explorer.Explorer {
	e, err := explorer.MountExplorer(EXPLORATION_LOG_FILE, explorer.RANDOM)
	if err != nil {
		l.Fatal(err)
	}
	e.SetMaxDepth(MAX_EXPLORATION_DEPTH)
	return e
}

func getDaraProcs() []explorer.ProcThread {
	threads := []explorer.ProcThread{}
	for i := 0; i < *procs; i++ {
		for _, routine := range procchan[i].Routines {
			if dara.GetDaraProcStatus(routine.Status) != dara.Idle {
				t := explorer.ProcThread{ProcID: i, Thread: routine}
				threads = append(threads, t)
			}
		}
	}
	return threads
}

func explore_sched() {
	e := explore_init()
	l.Println("Dora the Explorer begins")
	LastProc = -1
	ProcID := roundRobin()
	var i int
	for i < EXPLORELEN {
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
							events := ConsumeAndPrint(ProcID)
							schedule = append(schedule,events...)
							if (i >= EXPLORELEN - *procs) {
								//mark the processes for death
								l.Printf("Ending Execution!")
								procchan[ProcID].Run = -4 //mark process for death
							} else {
								procchan[ProcID].Run = -1	//lock process
							}
							procs := getDaraProcs()
							nextProc := e.GetNextThread(procs,events)
							ProcID = nextProc.ProcID
							procchan[ProcID].Run = nextProc.Thread.Gid
							procchan[ProcID].RunningRoutine = nextProc.Thread
							i++
							flag = false
						}
						//l.Print("Still running")
						atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED)
					}
					time.Sleep(time.Microsecond)
				}
			}
		}
	}
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
	p , err = runtime.Mmap(nil,
			       dara.CHANNELS*dara.DARAPROCSIZE,
                               dara.PROT_READ|dara.PROT_WRITE ,dara.MAP_SHARED,dara.DARAFD,0)

	if err != 0 {
		log.Println(err)
		l.Fatal(err)
	}

	//Map control struct into shared memory
	procchan = (*[dara.CHANNELS]dara.DaraProc)(p)
	//rand.Seed(int64(time.Now().Nanosecond()))
	//var count int
	for i:=range procchan {
		procchan[i].Lock = dara.UNLOCKED
		procchan[i].SyscallLock = dara.UNLOCKED
		procchan[i].Run = -1
		procchan[i].Syscall = -1
	}
	//State = RECORD
	if *replay {
		l.Println("Started replaying")
		replay_sched()
		l.Println("Finished replaying")
	} else if *record {
		l.Println("Started recording")
		record_sched()
		l.Println("Finished recording")
	} else if *explore {
		explore_sched()
		l.Println("Finished exploring")
	}
	l.Println("Exiting scheduler")
}


//Printing Functions
