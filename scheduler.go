package main

import (
	"fmt"
	"os"
	"net"
	"sync/atomic"
	"runtime"
	//"math/rand"
	//"runtime/internal/atomic"
	"encoding/json"
	"unsafe"
	"flag"
	"time"
	"log"
)

//Command line arguments
var (
	procs = flag.Int("procs", 1, "The number of processes to model check")
	record = flag.Bool("w", false, "Record an execution")
	replay = flag.Bool("r", false, "Replay an execution")
	manual = flag.Bool("m", false, "Manually step through an execution using a udp connection into the scheduler")
)

//*****************************************************************/
// 					BEGIN SHARED VARIABLES

//Varables and constants in the follow section are defined both in the
//runtime, and in the global scheduler. The MUST agree, if not
//commucation through shared memory with be missaligend.

//These Constants are the arguments for MMAP, not all of the arguments
//are used, but are here for completeness

//*****************************************************************/

const(
	_EINTR  = 0x4
	_EAGAIN = 0xb
	_ENOMEM = 0xc

	_PROT_NONE  = 0x0
	_PROT_READ  = 0x1
	_PROT_WRITE = 0x2
	_PROT_EXEC  = 0x4

	_MAP_ANON    = 0x20
	_MAP_PRIVATE = 0x2
	_MAP_FIXED   = 0x10

	_MAP_SHARED = 0x01
)

//Constants for shared memory. These constants are also in the go
//runtime in runtime/proc.go, they must agree or communication between
//runttime and global scheduler will be missaligend. If you change one
//you must change the other!
const(
	//The number of prealocated communication channles in shared
	//memory. This value can change in the future, for now 5 is the
	//maximum number of Processes. Invariant: CHANNELS >= procs. TODO
	//assert this
	CHANNELS = 5
	//File discriptor for shared memory. This is set in the runscript.
	//TODO paramatrize this, or set it as a global variable
	DARAFD = 666

	//State of spin locks. Thsese are used cas operations to control
	//the execution of the insturmented runtimes
	UNLOCKED = 0
	LOCKED = 1

	//The max number of goroutines any process can have. This is used
	//for allocating shared memeory. In the future this may need to be
	//expaneded for the moment it is intended to be a generous bound.
	MAXGOROUTINES = 4096

	//The total size of the shared memory region is
	//PAGESIZE*SHAREDMEMPAGES
	PAGESIZE = 4096
	SHAREDMEMPAGES = 65536

	//The length of the schedule. When recording the system will
	//execute upto SCHEDLEN. The same is true on replay
	RECORDLEN = 1000
)	


//Goroutine states from runtime/proc.go
const (
	_Gidle = iota 		// 0
	_Grunnable 			// 1
	_Grunning 			// 2
	_Gsyscall 			// 3
	_Gwaiting 			// 4
	_Gmoribund_unused 	// 5
	_Gdead 				// 6
	_Genqueue_unused 	// 7
	_Gcopystack 		// 8
	_Gscan         = 0x1000
	_Gscanrunnable = _Gscan + _Grunnable // 0x1001
	_Gscanrunning  = _Gscan + _Grunning  // 0x1002
	_Gscansyscall  = _Gscan + _Gsyscall  // 0x1003
	_Gscanwaiting  = _Gscan + _Gwaiting  // 0x1004
)

//array of status to string from runtime/proc.go
var gStatusStrings = [...]string{
	_Gidle:      "idle",
	_Grunnable:  "runnable",
	_Grunning:   "running",
	_Gsyscall:   "syscall",
	_Gwaiting:   "waiting",
	_Gdead:      "dead",
	_Gcopystack: "copystack",
}

//DaraProc is used to communicate control and data information between
//a single instrumented go runtime and the global scheduler. One of
//these strucures is mapped into shared memory for each process that
//is launched during an execution. If there are more runtimes active
//Than DaraProcs the additional runtimes will not be controlled by the
//global scheduler, or Segfault immidiatly 
type DaraProc struct {

	//Lock is used to controll the execution of a process. A process
	//which is running but Not scheduled will spin on this lock using
	//checkandset operations If the lock is held The owner can modify
	//the state of the DaraProc
	Lock int32
	
	//Run is a depricated var with multipul purposes. Procs set their
	//Run to -1 when they Are done running (in replay mode) to let the
	//scheduler know they are done. The global scheduler sets this
	//value to 2 to let the runtime know its replay, and 3 for record
	//1 is used to denote the first event, and 0 indicates this
	//variable has not been initalized Originally Run was intended to
	//report the id of the goroutine that was executed, but that was
	//not always the same so the program counter  was needed, now
	//RunningRoutine is used to report this
	Run int
	//RunningRoutine is the goroutine scheduled, running, or ran, for
	//any single replayed event in a schedule. In Record, the
	//executed goroutine is reported via this variable, in Replay the
	//global scheduler tells the runtime which routine to run with
	//RunningRoutine
	RunningRoutine RoutineInfo
	//Routines is the full set of goroutines a process is allowed to
	//run. The total number is allocated upfront so that shared memory
	//does not need to be resized dynamically. After each itteration
	//of scheduling runtimes update the states of all their routines
	//via this structure
	Routines [MAXGOROUTINES]RoutineInfo
}

//RoutineInfo contains data specific to a single goroutine
type RoutineInfo struct {
	//Set to one of the statuses in the constant block above
	Status uint32
	//Goroutine id as set by the runtime. This is sometimes usefull
	//for detecting which routine is which, but it is not always the
	//same between runs. However 1 is allways main, while 2 is a
	//finalizer, and 3 is a garbage collection invocator.
	Gid int
	//Program counter that this goroutine was launched from
	Gpc uintptr
	//A count of how many other goroutines were launched on the same
	//pc prior to this goroutine. (Gpc,Routinecount) is a unique id
	//for a goroutine on a given processor.
	RoutineCount int
	//A textual discription of the function this goroutine was forked
	//from.In the future it can be removed.
	FuncInfo [64]byte
}

func (ri *RoutineInfo) String() string {
	return fmt.Sprintf("[Status: %s Gid: %d Gpc: %d Rc: %d F: %s]",gStatusStrings[(*ri).Status],(*ri).Gid,(*ri).Gpc,(*ri).RoutineCount, string((*ri).FuncInfo[:64]))
}

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
	procchan *[CHANNELS]DaraProc
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
	//global schedule
	//TODO make all schedule operations object calls on the scedule
	//(ie schedule.next())
	schedule Schedule
)

//Event is a wrapper for a single scheduling event
type Event struct {
	//ProcID is an ID for a single runtime
	ProcID int
	//Routine contians information about the routine to run, including
	//state, blocking, and id information
	Routine RoutineInfo
}

func (e *Event) String() string {
		return fmt.Sprintf("[ProcID %d, %s]\n",(*e).ProcID,(*e).Routine.String())
}

//Type which encapsulates a single schedule
//TODO integerate with vaastav to build a single schedule for DPOR
type Schedule []Event

//Get Schedule string
//TODO make this fast with buffers
func (s *Schedule) String() string{
	var output string
	for i := range *s {
		output += (*s)[i].String()
	}
	return output
}

func checkargs() {
	flag.Parse()
	//Set exactly one arg for record or replay
	if *record && *replay {
		l.Fatal("enable either replay or record, not both")
	}
	if !(*record || *replay) {
		l.Fatal("enable replay or record")
	}
	if *record {
		l.Print("Recording")
	} else {
		l.Print("Replaying")
	}
	return
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
	p , err = runtime.Mmap(nil,SHAREDMEMPAGES*PAGESIZE,_PROT_READ|_PROT_WRITE ,_MAP_SHARED,DARAFD,0)

	//TODO can probably be deleted
	time.Sleep(time.Second * 1)

	//Map control struct into shared memory
	procchan = (*[CHANNELS]DaraProc)(p)
	//rand.Seed(int64(time.Now().Nanosecond()))
	//var count int
	for i:=range procchan {
		procchan[i].Lock = UNLOCKED
		procchan[i].Run = -1
	}
	//Initalize Processes scheduling with scheduling type
	LastProc = -1
	ProcID := roundRobin()
	
	//-----------------REPLAY-----------------//
	if *replay {
		var i int // itterator for schedule
		//Read in schedule from disk
		f, err := os.Open("Schedule.json")
		if err != nil {
			//TODO print out a reasonable error message along with the
			//fatal
			//TODO take the Scedule as a command line arg
			l.Println("Unable to read Schedule.json from current directory")
			l.Fatal(err)
		}
		//Decode scedule from json encoding
		dec := json.NewDecoder(f)
		err = dec.Decode(&schedule)
		if err != nil {
			l.Fatal(err)
		}
		for i<len(schedule) {
			//l.Println("RUNNING SCHEDULER")

			//else busy wait
			if atomic.CompareAndSwapInt32(&(procchan[schedule[i].ProcID].Lock),UNLOCKED,LOCKED) {
				if procchan[schedule[i].ProcID].Run == -1 { //TODO check predicates on goroutines + schedule

					//move forward
					forward()

					//l.Print("Scheduling Event")
					//TODO send a proper signal to the runtime with
					//the goroutine ID that needs to be run
					runnable := make([]int,0)
					for j, info := range procchan[schedule[i].ProcID].Routines {
						if info.Status != _Gidle {
							l.Printf("Proc[%d]Routine[%d].Info = %s",schedule[i].ProcID,j,info.String())
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
					l.Printf("Running (%d/%d) %d %s",i,len(schedule),schedule[i].ProcID,schedule[i].Routine.String())

					//l.Printf("procchan[schedule[%d]].Run = %d",i,procchan[schedule[i]].Run)
					//TODO explore schedule if in explore mode?

					//l.Print("unLocking store")
					atomic.StoreInt32(&(procchan[schedule[i].ProcID].Lock),UNLOCKED)
					//l.Print(procchan)

					for {
					//l.Printf("Busy waiting")
						if atomic.CompareAndSwapInt32(&(procchan[schedule[i].ProcID].Lock),UNLOCKED,LOCKED) { 
						//l.Print("go routine locked again")
							//l.Printf("procchan[schedule[%d]].Run = %d",i,procchan[schedule[i]].Run)
							if procchan[schedule[i].ProcID].Run == -1 {
								//l.Print("Job Done!")
								atomic.StoreInt32(&(procchan[schedule[i].ProcID].Lock),UNLOCKED)
								//l.Print(procchan)
								i++
								break
							}
							//Preemtion is turned off so this should
							//never happen.
							atomic.StoreInt32(&(procchan[schedule[i].ProcID].Lock),UNLOCKED)
							//TODO log.Fatalf("Preemtion is turned on")
						}
						//time.Sleep(time.Second)
						
					}
				}
			}
		}
	} else if *record {
		var i int
		for i<RECORDLEN {
			//l.Println("RUNNING SCHEDULER")
			//else busy wait
			if atomic.CompareAndSwapInt32(&(procchan[ProcID].Lock),UNLOCKED,LOCKED) {
				if procchan[ProcID].Run == -1 { //TODO check predicates on goroutines + schedule

					forward()

					runningindex := -3
					//l.Printf("Running On P %d",ProcID)
					procchan[ProcID].Run = runningindex 

					//l.Print("unLocking store")
					atomic.StoreInt32(&(procchan[ProcID].Lock),UNLOCKED)
					//l.Print(procchan)
					l.Printf("Recording Event %d",i)

					for {
								//l.Print(procchan)
						if atomic.CompareAndSwapInt32(&(procchan[ProcID].Lock),UNLOCKED,LOCKED) { 
							//l.Printf("procchan[schedule[%d]].Run = %d",i,procchan[schedule[i]].Run)
							if procchan[ProcID].Run != -3 {
								//the process is done running
								//record which goroutine it let
								//run
								//ran := procchan[ProcID].Run
								ri := procchan[ProcID].RunningRoutine
								e := Event{ProcID,ri}
								l.Printf("Ran: %s",e.String())
								schedule = append(schedule,e)
								procchan[ProcID].Run = -1
								ProcID = roundRobin()
								atomic.StoreInt32(&(procchan[ProcID].Lock),UNLOCKED)
								i++
								break
							}
							//l.Print("Still running")
							atomic.StoreInt32(&(procchan[ProcID].Lock),UNLOCKED)
						}
						time.Sleep(time.Microsecond)
						
					}
				}
			}
			//Here the schedule is written out after every itteration
			//of the record (exploration) stage. This is done so that
			//if the record phase fails, all of the schedule up till
			//this point is recoreded, and can be repayed to find the
			//root cause of the error.
			f, erros := os.Create("Schedule.json")
			if erros != nil {
				l.Fatal(err)
			}
			enc := json.NewEncoder(f)
			enc.Encode(schedule)
		}
		l.Printf(schedule.String())

	}
}

//Move the scedule forward one event
func forward() {
	//In the manual case read from udp. This should just be a return
	//read from another terminal from ui.go
	if *manual {
		udpl.ReadFrom(udpb)
		l.Print(udpb)
	} else {
		//Sleep for a short period of time
		//TODO work towards getting this to 0, i.e remove all timing
		//bugs
		time.Sleep(100 *time.Millisecond)
	}
}

//Round robin scheduling alorithm, uses and sets globals LastProc, and
//procs
func roundRobin() int {
	LastProc = (LastProc + 1) % (*procs+1)
	if LastProc == 0 {
		LastProc++
	}
	return LastProc
}
