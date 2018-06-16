package common

import (
	"fmt"
)

//Goroutine states from runtime/proc.go
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

type DaraProcStatus uint32

const (
	Idle DaraProcStatus = iota
	Runnable
	Running
	Syscall
	Waiting
	Moribound_Unused
	Dead
	Enqueue_Unused
	Copystack
	Scan DaraProcStatus = 0x1000
	ScanRunnable = Scan + Runnable
	ScanRunning  = Scan + Running
	ScanSyscall  = Scan + Syscall
	ScanWaiting  = Scan + Waiting
)

func GetDaraProcStatus (status uint32) DaraProcStatus {
	return DaraProcStatus(status)
}

const (
	//The max number of goroutines any process can have. This is used
	//for allocating shared memeory. In the future this may need to be
	//expaneded for the moment it is intended to be a generous bound.
	MAXGOROUTINES = 4096
)

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
	Syscall int
}


func (ri *RoutineInfo) String() string {
	return fmt.Sprintf("[Status: %s Gid: %d Gpc: %d Rc: %d F: %s]",gStatusStrings[(*ri).Status],(*ri).Gid,(*ri).Gpc,(*ri).RoutineCount, string((*ri).FuncInfo[:64]))
}

/**
  * Struct representing an event in the lifetime of a process
  */
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

