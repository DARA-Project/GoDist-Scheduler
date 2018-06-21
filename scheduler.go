package main

import (
	"dara"
	"encoding/json"
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
)

//Command line arguments
var (
	procs = flag.Int("procs", 1, "The number of processes to model check")
	record = flag.Bool("w", false, "Record an execution")
	replay = flag.Bool("r", false, "Replay an execution")
	explore = flag.Bool("e", false, "Explore a recorded execution")
	manual = flag.Bool("m", false, "Manually step through an execution using a udp connection into the scheduler")
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
	//buffer for reading from the manual event stepper, the values
	//read into the buffer really don't matter yet
	//TODO allow users to specifcy scheduling decisions based on udp
	//stepping
	udpb []byte
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
		if (i == len(schedule) - 1 || i == len(schedule)) {
			l.Println("Hmm")
		}
		//l.Println("RUNNING SCHEDULER")
		//else busy wait
		if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].ProcID].Lock))),dara.UNLOCKED,dara.LOCKED) {
			if procchan[schedule[i].ProcID].Run == -1 { //TODO check predicates on goroutines + schedule

				//move forward
				forward()

				//l.Print("Scheduling Event")
				//TODO send a proper signal to the runtime with
				//the goroutine ID that needs to be run
				runnable := make([]int,0)
				for j, info := range procchan[schedule[i].ProcID].Routines {
					if dara.GetDaraProcStatus(info.Status) != dara.Idle {
						l.Printf("Proc[%d]Routine[%d].Info = %s",schedule[i].ProcID,j,common.PrintRoutineInfo(&info))
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
					//l.Print("first instance")
					runningindex = -2
				} else {
					//replay schedule
					runningindex = schedule[i].Routine.Gid
				}
				//Assign which goroutine to run
				procchan[schedule[i].ProcID].Run = runningindex //TODO make this the scheduled GID
				procchan[schedule[i].ProcID].RunningRoutine = schedule[i].Routine //TODO make this the scheduled GID
				l.Printf("Running (%d/%d) %d %s",i,len(schedule)-1,schedule[i].ProcID,common.PrintRoutineInfo(&schedule[i].Routine))

				//l.Printf("procchan[schedule[%d]].Run = %d",i,procchan[schedule[i]].Run)
				//TODO explore schedule if in explore mode?

				//l.Print("unLocking store")
				atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].ProcID].Lock))),dara.UNLOCKED)
				//l.Print(procchan)

				for {
					if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].ProcID].Lock))),dara.UNLOCKED,dara.LOCKED) { 
						if procchan[schedule[i].ProcID].Run == -1 {
							//l.Print("Job Done!")
							atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].ProcID].Lock))),dara.UNLOCKED)
							//l.Print(procchan)
							i++
							break
						}
						atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].ProcID].Lock))),dara.UNLOCKED)
						//TODO log.Fatalf("Preemtion is turned on")
					}
					//time.Sleep(time.Second)
				}
			}
		}
	}
	//End computation
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
				l.Printf("Recording Event %d",i)

				for {
							//l.Print(procchan)
					if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED,dara.LOCKED) { 
						//l.Printf("procchan[schedule[%d]].Run = %d",i,procchan[schedule[i]].Run)
						if procchan[ProcID].Run != -3 {
							//Update the last running routine
							ri := procchan[ProcID].RunningRoutine
							e := dara.Event{ProcID,ri}
							l.Printf("Ran: %s",common.PrintEvent(&e))
							schedule = append(schedule,e)
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
							atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED)
							break
						}
						//l.Print("Still running")
						atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED)
					}
					time.Sleep(time.Microsecond)
				}
			}
		}
		f, erros := os.Create("Schedule.json")
		if erros != nil {
			l.Fatal(err)
		}
		enc := json.NewEncoder(f)
		enc.Encode(schedule)
	}
	l.Printf(common.PrintSchedule(&schedule))
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
		procchan[i].Lock = dara.UNLOCKED
		procchan[i].Run = -1
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
