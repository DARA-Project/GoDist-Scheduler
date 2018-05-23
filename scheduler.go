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

	"time"

	"log"
)

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

	//ADDED
	_MAP_SHARED = 0x01

	CHANNELS = 5
	DARAFD = 666

	UNLOCKED = 0
	LOCKED = 1

	SCHEDLEN = 5
	PROCS = 3

	MAXGOROUTINES = 4096

	PAGESIZE = 4096
	SHAREDMEMPAGES = 65536

	RECORD = 0
	REPLAY = 1

	RECORDLEN = 500
	
	ROUNDROBIN = 0
)

const (
	_Gidle = iota // 0
	_Grunnable // 1
	_Grunning // 2
	_Gsyscall // 3
	_Gwaiting // 4
	_Gmoribund_unused // 5
	_Gdead // 6
	_Genqueue_unused // 7
	_Gcopystack // 8
	_Gscan         = 0x1000
	_Gscanrunnable = _Gscan + _Grunnable // 0x1001
	_Gscanrunning  = _Gscan + _Grunning  // 0x1002
	_Gscansyscall  = _Gscan + _Gsyscall  // 0x1003
	_Gscanwaiting  = _Gscan + _Gwaiting  // 0x1004
)


var gStatusStrings = [...]string{
	_Gidle:      "idle",
	_Grunnable:  "runnable",
	_Grunning:   "running",
	_Gsyscall:   "syscall",
	_Gwaiting:   "waiting",
	_Gdead:      "dead",
	_Gcopystack: "copystack",
}


type DaraProc struct {
	Lock int32
	Run int
	TotalRoutines int
	RunningRoutine RoutineInfo
	Routines [MAXGOROUTINES]RoutineInfo
}

type RoutineInfo struct {
	Status uint32
	Gid int
	Gpc uintptr
	RoutineCount int
	FuncInfo [64]byte
}

func (ri *RoutineInfo) String() string {
	return fmt.Sprintf("[Status: %s Gid: %d Gpc: %d Rc: %d F: %s]",gStatusStrings[(*ri).Status],(*ri).Gid,(*ri).Gpc,(*ri).RoutineCount, string((*ri).FuncInfo[:64]))
}

var (
	p unsafe.Pointer
	err int
	procchan *[CHANNELS]DaraProc
	State int
	ScheduleType int
	LastProc int
)

var (
	l *log.Logger
	udpl net.PacketConn
	udpb []byte
)


type Event struct {
	ProcID int
	Routine RoutineInfo
}

func (e *Event) String() string {
		return fmt.Sprintf("[ProcID %d, %s]\n",(*e).ProcID,(*e).Routine.String())
}

type Schedule []Event

func (s *Schedule) String() string{
	var output string
	for i := range *s {
		output += (*s)[i].String()
	}
	return output
}


func main() {
	l = log.New(os.Stdout,"[Scheduler]",log.Lshortfile)
	l.Println("Starting the Scheduler")

	var errudp error
	udpl, errudp = net.ListenPacket("udp", "localhost:6666")
	udpb = make([]byte,128)
	if errudp !=nil {
		l.Fatal(errudp)
	}


	//TODO Document and use better cmd line args
	state := os.Args[1]
	if state == "-w" {
		State = RECORD
	} else if state == "-r" {
		State = REPLAY
	} else {
		l.Fatal("Please specify operation mode (-w record, -r replay)")
	}

	//Init MMAP (this should be moved to the init function
	p , err = runtime.Mmap(nil,SHAREDMEMPAGES*PAGESIZE,_PROT_READ|_PROT_WRITE ,_MAP_SHARED,DARAFD,0)

	time.Sleep(time.Second * 1)

	procchan = (*[CHANNELS]DaraProc)(p)
	//rand.Seed(int64(time.Now().Nanosecond()))
	//var count int
	for i:=range procchan {
		procchan[i].Lock = UNLOCKED
		procchan[i].Run = -1
	}
	//State = RECORD
	ScheduleType = ROUNDROBIN
	LastProc = -1
	ProcID := roundRobin()
	
	if State == REPLAY {
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
			//l.Println("RUNNING SCHEDULER")

			//else busy wait
			if atomic.CompareAndSwapInt32(&(procchan[schedule[i].ProcID].Lock),UNLOCKED,LOCKED) {
				if procchan[schedule[i].ProcID].Run == -1 { //TODO check predicates on goroutines + schedule
					l.Print("GET GET GET GET GOT GOT GOT\n")

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
							//l.Print("Still running")
							atomic.StoreInt32(&(procchan[schedule[i].ProcID].Lock),UNLOCKED)
						}
						//time.Sleep(time.Second)
						
					}
				}
			}
		}
	} else if State == RECORD {
		var i int
		f, err := os.Create("Schedule.json")
		if err != nil {
			l.Fatal(err)
		}
		enc := json.NewEncoder(f)
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
		}
		enc.Encode(schedule)
		l.Printf(schedule.String())

	}
}

func forward() {
	if false {
		udpl.ReadFrom(udpb)
		l.Print(udpb)
	} else {
		time.Sleep(100 *time.Millisecond)
	}
}

var schedule Schedule

func roundRobin() int {
	LastProc = (LastProc + 1) % (PROCS+1)
	if LastProc == 0 {
		LastProc++
	}
	return LastProc
}

func populateSchedule() []int {
	s := make([]int,SCHEDLEN)
	for i:=0;i<SCHEDLEN;i++ {
		s[i]=i%PROCS + 1
	}
	return s
}

