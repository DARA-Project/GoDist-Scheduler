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

/**
  * Struct representing a Process/Node in the system
  */
type DaraProc struct {
	Lock int32
	Run int
	TotalRoutines int
	RunningRoutine RoutineInfo
	Routines [MAXGOROUTINES]RoutineInfo
}

/**
  * Struct representing all the information for a goroutine
  */
type RoutineInfo struct {
	Status uint32
	Gid int
	Gpc uintptr
	RoutineCount int
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

