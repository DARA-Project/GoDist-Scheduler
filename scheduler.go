package main

import (
	"bytes"
	"dara"
	"encoding/json"
	"flag"
	"github.com/DARA-Project/GoDist-Scheduler/common"
	"github.com/DARA-Project/GoDist-Scheduler/explorer"
	"github.com/DARA-Project/GoDist-Scheduler/propchecker"
	"log"
    "net/rpc"
	"os"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"
	"unsafe"
	"fmt"
)

type ProcStatus int

const (
	ALIVE ProcStatus = iota
	DEAD
)

//Command line arguments
var (
	procs      = flag.Int("procs", 1, "The number of processes to model check")
	record     = flag.Bool("record", false, "Record an execution")
	replay     = flag.Bool("replay", false, "Replay an execution")
	explore    = flag.Bool("explore", false, "Explore a recorded execution")
	sched_file = flag.String("schedule", "", "The schedule file where the schedule will be stored/loaded from")
	strategy   = flag.String("strategy", "random", "The strategy to be used for exploration. One of [random|unique|frequency|nodefrequency]")
	maxdepth   = flag.Int("maxdepth", 100, "The maximum depth for an exploration path")
	maxruns    = flag.Int("maxruns", 2, "The maximum number of exploration runs to be performed")
)

const (
	//The length of the schedule. When recording the system will
	//execute up to dara.SCHEDLEN. The same is true on replay
	RECORDLEN             = 10000
	EXPLORELEN            = 100
	EXPLORATION_LOG_FILE  = "visitedLog.txt"
    // TODO: Make this an option
	OVERLORD_RPC_SERVER = "0.0.0.0:45000"
)

var (
	MAX_EXPLORATION_DEPTH = 100
	MAX_EXPLORE_RUNS = 1
)

//Global variables specific to shared memory
var (
	//pointer to shared memory, unsafe so that it can be casted
	p unsafe.Pointer
	//err is an integer, because it is set from errors in the runtime
	//where there is no error type set
	err int
	//Procchan is the communication channel laid over shared memory
	procchan *[dara.CHANNELS]dara.DaraProc
)

// Global variables for managing the state of Dara processes
var (
	//Used to determine which process (runtime) will be run on the
	//next scheduling decision when in record mode
	LastProc int
	LogLevel int
	//Variable to hold propertychecker
	checker *propchecker.Checker
	// Variable to hold the logger
	l *log.Logger
	// Variable where the schedule is stored
	// This might be the input scheduler for replay/explore or the output schedule for record
	schedule dara.Schedule
	// Monotonous event counter
	eventCounter int

	// Map that maps the Dara Process ID to its status
	procStatus map[int]ProcStatus
	// Map that maps the Dara Process ID to whether we own the procchan lock for that process
	lockStatus map[int]bool
	// Map that maps Dara Process ID to linux PID
	linuxpids map[int]uint64
	// Proc Coverage
	allCoverage map[int]*explorer.CovStats
	// Virtual Clocks
	clocks    map[int]*explorer.VirtualClock
)

// decodeInputSchedule decodes the input schedule for replay/exploration use
func decodeInputSchedule() *dara.Schedule {
	var schedule dara.Schedule
	f, err := os.Open(*sched_file)
	if err != nil {
		l.Fatal(err)
	}
	dec := json.NewDecoder(f)
	err = dec.Decode(&schedule)
	if err != nil {
		l.Fatal(err)
	}
	return &schedule
}

// checkargs checks and parses the command line arguments
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
		if *strategy != "random" && *strategy != "unique" && *strategy != "frequency" && *strategy != "nodefrequency" {
			l.Fatal("Invalid exploration strategy " + *strategy + " specified")
		}
	} else {
		l.Print("Replaying")
	}
	level := os.Getenv("DARA_LOG_LEVEL")
	i, err := strconv.Atoi(level)
	if err != nil {
		l.Fatal("Invalid log level specified")
	}
	MAX_EXPLORATION_DEPTH = *maxdepth
	MAX_EXPLORE_RUNS = *maxruns
	LogLevel = int(i)
	return
}

func forward() {
	time.Sleep(1 * time.Millisecond)
}

// dprint is a log level print function for the Global Scheduler
func dprint(level int, pfunc func()) {
	if LogLevel == dara.OFF {
		return
	}

	if level >= LogLevel {
		pfunc()
	}
}

// roundRobin selects the next DaraProcess to be run.
// Currently, it follows a round robin strategy (well no shit)
func roundRobin() int {
	flag := false
	for !flag {
		LastProc = (LastProc + 1) % (*procs + 1)
		// Dara doesn't have ProcID 0
		if LastProc == 0 {
			LastProc++
		}
		dprint(dara.DEBUG, func() {l.Println("Checking status of", LastProc, "which is", procStatus[LastProc])})
		if procStatus[LastProc] == ALIVE {
			dprint(dara.DEBUG, func() {l.Println("Next process should be", LastProc)})
			flag = true
		}
	}
	return LastProc
}

// getSchedulingEvents returns all the scheduling events in a schedule
func getSchedulingEvents(sched *dara.Schedule) map[int][]dara.Event {
	proc_event_map := make(map[int][]dara.Event)
	for i := 0; i < len(sched.LogEvents); i++ {
		eventTypeString := common.EventTypeString(sched.LogEvents[i].Type)
		if eventTypeString == "SCHEDULE" {
			// Don't want to run timer events
			if string(bytes.Trim(sched.LogEvents[i].G.FuncInfo[:64], "\x00")[:]) != "runtime.timerproc" {
				proc_event_map[sched.LogEvents[i].P] = append(proc_event_map[sched.LogEvents[i].P], sched.LogEvents[i])
			}
		}
	}
	return proc_event_map
}

// ConsumeCoverage consumes the coverage information for this run
func ConsumeCoverage(ProcID int) explorer.CovStats {
    coverage := make(explorer.CovStats)
    for i := 0; i < procchan[ProcID].CoverageIndex; i++ {
        blockID := string(bytes.Trim(procchan[ProcID].Coverage[i].BlockID[:dara.BLOCKIDLEN], "\x00")[:])
        count := procchan[ProcID].Coverage[i].Count
        coverage[blockID] = count
    }
    // Reset the CoverageIndex
    procchan[ProcID].CoverageIndex = 0
    return coverage
}

// ConsumeAndPrint calls ConsumeLog
// TODO: Maybe we can delete this function and call ConsumeLog directly
func ConsumeAndPrint(ProcID int, context *map[string]interface{}) []dara.Event {
	cl := ConsumeLog(ProcID, context)
	dprint(dara.DEBUG, func() {l.Println("Consumed", len(cl), "events")})
	return cl
}

// ConsumeLog consumes all the events from the shared memory and produces
// a list of events for further use
func ConsumeLog(ProcID int, context *map[string]interface{}) []dara.Event {
	dprint(dara.DEBUG, func() { l.Println("Current log event index is", procchan[ProcID].LogIndex, "for Process", ProcID) })
	logsize := procchan[ProcID].LogIndex
	dprint(dara.DEBUG, func() { l.Println("Current log event index is", logsize, "for Process", ProcID) })
	log := make([]dara.Event, logsize)
	for i := 0; i < logsize; i++ {
		ee := &(procchan[ProcID].Log[i])
		e := &(log[i])
		(*e).Type = (*ee).Type
		(*e).P = (*ee).P
		(*e).G = (*ee).G
		(*e).Epoch = (*ee).Epoch
		(*e).LE.LogID = string(bytes.Trim((*ee).ELE.LogID[:dara.VARBUFLEN], "\x00")[:])
		(*e).LE.Vars = make([]dara.NameValuePair, (*ee).ELE.Length)
		for j := 0; j < len((*e).LE.Vars); j++ {
			(*e).LE.Vars[j].VarName = string(bytes.Trim((*ee).ELE.Vars[j].VarName[:dara.VARBUFLEN], "\x00")[:])
			if e.Type == dara.LOG_EVENT {
				// This is a log event so we need to update the context with new variable:value pairs
				(*e).LE.Vars[j].Value = runtime.DecodeValue((*ee).ELE.Vars[j].Type, (*ee).ELE.Vars[j].Value)
				(*e).LE.Vars[j].Type = string(bytes.Trim((*ee).ELE.Vars[j].Type[:dara.VARBUFLEN], "\x00")[:])
				// Build Up context for property checking
				(*context)[e.LE.Vars[j].VarName] = e.LE.Vars[j].Value
			} else if e.Type == dara.DELETEVAR_EVENT {
				// Remove Variable from the context as the application asked for something to be deleted from
				// the context
				delete(*context, e.LE.Vars[j].VarName)
			}
		}
		(*e).SyscallInfo = (*ee).SyscallInfo
		dprint(dara.DEBUG, func() { l.Println(common.ConciseEventString(e)) })
		if e.Type == dara.THREAD_EVENT {
			dprint(dara.DEBUG, func() { l.Println("Thread creation noted with ID", e.G.Gid, "at function", string(e.G.FuncInfo[:64])) })
		}
		if e.Type == dara.SCHED_EVENT {
			dprint(dara.DEBUG, func() { l.Println("Scheduling event for Process", ProcID, "and goroutine", common.RoutineInfoString(&e.G))})
		}
		if e.Type == dara.TIMER_EVENT {
			// A timer was installed! Put this on the vclock queue.
			if v, ok := clocks[ProcID]; ok {
				eventCounter += 1
				v.AddTimerEvent(eventCounter, explorer.Timer, uint64(e.SyscallInfo.Args[1].Integer64), uint64(e.SyscallInfo.Args[2].Integer64), int64(e.G.Gid), e.SyscallInfo.Args[0].Integer64)
			} else {
				dprint(dara.WARN, func() {l.Println("Virtual Clock not found for Process: ", ProcID)} )
			}
		}
		if e.SyscallInfo.SyscallNum == dara.DSYS_SLEEP {
			// A goroutine was put to sleep. It will wake back up ....when the explorer decides
			if v, ok := clocks[ProcID]; ok {
				eventCounter += 1
				v.AddTimerEvent(eventCounter, explorer.Sleep, uint64(e.SyscallInfo.Args[0].Integer64), 0, e.SyscallInfo.Args[1].Integer64, 0)
			} else {
				dprint(dara.WARN, func() {l.Println("Virtual Clock not found for Process: ", ProcID)} )
			}
		}
	}
	procchan[ProcID].LogIndex = 0
	procchan[ProcID].Epoch++
	return log
}

// CheckAllProcsDead checks if all local runtime/processes have died/completed or not.
func CheckAllProcsDead() bool {
	for _, status := range procStatus {
		if status != DEAD {
			return false
		}
	}
	return true
}

// CheckAllGoRoutinesDead returns true if all the goroutines in local runtime have died or not.
func CheckAllGoRoutinesDead(ProcID int) bool {
	allDead := true
	dprint(dara.DEBUG, func() { l.Printf("Checking if all goroutines are dead yet for Process %d\n", ProcID) })
	//l.Printf("Num routines are %d", len(procchan[ProcID].Routines))
	for _, routine := range procchan[ProcID].Routines {
		// Only check for dead goroutines for real go routines
		if routine.Gid != 0 {
			status := dara.GetDaraProcStatus(routine.Status)
			dprint(dara.DEBUG, func() { l.Printf("Status of GoRoutine %d is %s\n", routine.Gid, dara.GStatusStrings[status]) })
			allDead = allDead && (status == dara.Dead)
            if status != dara.Dead {
                //dprint(dara.INFO, func() { l.Printf("GoRoutine %d Proc %d is not dead yet\n", routine.Gid, ProcID)})
                return false
            }
		}
	}
	return allDead
}

// GetAllRunnableRoutines returns all the goroutines that were runnable when
// we switched back to the global scheduler
func GetAllRunnableRoutines(event dara.Event) []int {
	runnable := make([]int, 0)
	for j, info := range procchan[event.P].Routines {
		if dara.GetDaraProcStatus(info.Status) != dara.Idle {
			dprint(dara.DEBUG, func() { l.Printf("Proc[%d]Routine[%d].Info = %s", event.P, j, common.RoutineInfoString(&info)) })
			runnable = append(runnable, j)
		}
	}
	return runnable
}

// IsEventReplayable checks if an event is replayable if the goid to be
// scheduled for that event is one of the runnable routines.
func IsEventReplayable(runnable []int, Gid int) bool {
	for _, id := range runnable {
		if Gid == id {
			return true
		}
	}
	return false
}

// CompareEvents compares if two events are the same or not.
// This is used in checking if replay is correct or not
// Note: Currently two events are the same if they are of the same type
func CompareEvents(e1 dara.Event, e2 dara.Event) bool {
	// TODO Make a more detailed comparison
	if e1.Type != e2.Type {
		return false
	}
	return true
}

// CheckEndOrCrash checks if the event is a crash or an ending event
func CheckEndOrCrash(e dara.Event) bool {
	if e.Type == dara.END_EVENT || e.Type == dara.CRASH_EVENT {
		return true
	}
	return false
}

// initExplorer initializes the explorer
func initExplorer(seed *dara.Schedule) *explorer.Explorer {
	var explorationStrat explorer.Strategy
	if *strategy == "random" {
		explorationStrat = explorer.RANDOM
	} else if *strategy == "unique" {
		explorationStrat = explorer.COVERAGE_UNIQUE
	} else if *strategy == "frequency" {
		explorationStrat = explorer.COVERAGE_FREQUENCY
	} else if *strategy == "nodefrequency" {
		explorationStrat = explorer.COVERAGE_NODE_FREQUENCY
	}
	e, err := explorer.MountExplorer(EXPLORATION_LOG_FILE, explorationStrat, seed, *procs, LogLevel)
	if err != nil {
		l.Fatal(err)
	}
	e.SetMaxDepth(MAX_EXPLORATION_DEPTH)
	return e
}

// getDaraProcs returns the list of all goroutines for a given process
func getDaraProcs(procID int) *[]explorer.ProcThread {
	threads := []explorer.ProcThread{}
	for _, routine := range procchan[procID].Routines {
		if dara.GetDaraProcStatus(routine.Status) != dara.Idle {
			t := explorer.ProcThread{ProcID: procID, Thread: routine}
			threads = append(threads, t)
		}
	}
	return &threads
}

func getAllDaraProcs() *[]explorer.ProcThread {
	threads := []explorer.ProcThread{}
	for i := 1; i <= *procs; i++ {
		for _, routine := range procchan[i].Routines {
			if dara.GetDaraProcStatus(routine.Status) != dara.Idle {
				t := explorer.ProcThread{ProcID: i, Thread: routine}
				threads = append(threads, t)
			}
		}
	}
	return &threads
}

// preloadReplaySchedule populates the shared memory with a pre-chosen schedule
func preloadReplaySchedule(proc_schedules map[int][]dara.Event) {
	for procID, events := range proc_schedules {
		for i, e := range events {
			procchan[procID].Log[i].Type = e.Type
			procchan[procID].Log[i].P = e.P
			procchan[procID].Log[i].G = e.G
		}
		procchan[procID].LogIndex = len(events)
	}
}

// checkProperties checks the user-defined properties under a given context
// that is collected using DaraLog calls in the local runtimes
func checkProperties(context map[string]interface{}) (bool, *[]dara.FailedPropertyEvent, error) {
	if checker != nil {
		return checker.Check(context)
	}
	return true, &[]dara.FailedPropertyEvent{}, nil
}

// replaySchedule replays a previously recorded/explored execution
func replaySchedule() {
	schedule := decodeInputSchedule()
	context := make(map[string]interface{})
	dprint(dara.DEBUG, func() { l.Println("Num Events : ", len(schedule.LogEvents)) })
	dprint(dara.DEBUG, func() { l.Println("Replaying the following schedule : ", common.ConciseScheduleString(&schedule.LogEvents)) })
	var i int
	fast_replay := os.Getenv("FAST_REPLAY")
	if fast_replay == "true" {
		proc_schedule_map := getSchedulingEvents(schedule)
		preloadReplaySchedule(proc_schedule_map)
		for procID := range procchan {
			if procchan[procID].Run == -1 && procchan[procID].LogIndex == 0 {
				procchan[procID].Run = -100
			} else {
				dprint(dara.INFO, func() { l.Println("Enabling runtime for proc", procID) })
				procchan[procID].Run = -5
			}
		}
		all_procs_done := false
		for !all_procs_done {
			all_procs_done = true
			for procID := range procchan {
				if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[procID].Lock))), dara.UNLOCKED, dara.LOCKED) {
					if procchan[procID].Run != -100 {
						// We haven't reached the end of replay for this runtime. Give up control to this runtime.
						all_procs_done = false
						//dprint(dara.INFO, func() {l.Println("Not over for Proc", procID, "with value", procchan[procID].Run)})
					}
					atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[procID].Lock))), dara.UNLOCKED)
				} else {
					all_procs_done = false
				}
			}
		}
		dprint(dara.INFO, func() { l.Println("Fast Replay done") })
	} else {
		for i < len(schedule.LogEvents) {
			dprint(dara.DEBUG, func(){l.Println("Replaying event index", i)})
			if CheckAllProcsDead() {
				dprint(dara.INFO, func() {l.Println("All processes dead but", len(schedule.LogEvents) - i, "events need to be replayed")})
				break
			}
			if lockStatus[schedule.LogEvents[i].P] || atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[schedule.LogEvents[i].P].Lock))), dara.UNLOCKED, dara.LOCKED) {
				if procchan[schedule.LogEvents[i].P].Run == -1 { //check predicates on goroutines + schedule

					//move forward
					forward()

					dprint(dara.DEBUG, func() { l.Println("Scheduling Event") })
					runnable := GetAllRunnableRoutines(schedule.LogEvents[i])
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
					check_replayable:
						can_run := IsEventReplayable(runnable, schedule.LogEvents[i].G.Gid)
						if !can_run {
							if string(bytes.Trim(schedule.LogEvents[i].G.FuncInfo[:64], "\x00")[:]) != "runtime.timerproc" {
								dprint(dara.INFO, func() { l.Println("G info was ", string(schedule.LogEvents[i].G.FuncInfo[:64])) })
								dprint(dara.FATAL, func() { l.Fatal("Can't run event ", i, ": ", common.ConciseEventString(&schedule.LogEvents[i])) })
							} else {
								//Trying to schedule timer thread. Let's just increment the event counter
								dprint(dara.INFO, func() { l.Println("G info was ", string(schedule.LogEvents[i].G.FuncInfo[:64])) })
								i++
								//Check if the new event is replayable
								goto check_replayable
							}
						}
						runningindex = schedule.LogEvents[i].G.Gid
					}
					_ = runningindex
					//Assign which goroutine to run
					procchan[schedule.LogEvents[i].P].Run = -5
					procchan[schedule.LogEvents[i].P].RunningRoutine = schedule.LogEvents[i].G
					dprint(dara.INFO, func() {
						l.Printf("Running (%d/%d) %d %s", i+1, len(schedule.LogEvents), schedule.LogEvents[i].P, common.RoutineInfoString(&schedule.LogEvents[i].G))
					})

					atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[schedule.LogEvents[i].P].Lock))), dara.UNLOCKED)
					for {
						if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[schedule.LogEvents[i].P].Lock))), dara.UNLOCKED, dara.LOCKED) {
							currentDaraProc := schedule.LogEvents[i].P
							if procchan[schedule.LogEvents[i].P].Run == -1 {
								atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[schedule.LogEvents[i].P].Lock))), dara.UNLOCKED)
								events := ConsumeAndPrint(currentDaraProc, &context)
                                coverage := ConsumeCoverage(currentDaraProc)
								dprint(dara.DEBUG, func() { l.Println("Replay Consumed ", len(events), "events") })
                                dprint(dara.DEBUG, func() { l.Println("Replay Consumed ", len(coverage), "blocks") })
								for _, e := range events {
									dprint(dara.DEBUG, func(){ l.Println(common.ConciseEventString(&e))})
									same_event := CompareEvents(e, schedule.LogEvents[i])
									if !same_event {
										dprint(dara.WARN, func() {
											l.Println("Replayed 2 different events", common.ConciseEventString(&e), common.ConciseEventString(&schedule.LogEvents[i]))
										})
									} else {
										dprint(dara.DEBUG, func() { l.Println("Replayed ", common.ConciseEventString(&schedule.LogEvents[i])) })
									}
									i++
								}
								dprint(dara.DEBUG, func() { l.Println("Replay : At event", i) })
								if i >= len(schedule.LogEvents) {
									dprint(dara.DEBUG, func() { l.Printf("Informing the local scheduler end of replay") })
									procchan[currentDaraProc].Run = -4
									break
								}
								break
							} else if procchan[schedule.LogEvents[i].P].Run == -100 {
								// This means that the local runtime finished and that we can move on
								currentProc := schedule.LogEvents[i].P
								events := ConsumeAndPrint(currentDaraProc, &context)
                                coverage := ConsumeCoverage(currentDaraProc)
                                dprint(dara.DEBUG, func() { l.Println("Replay Consumed ", len(coverage), "blocks") })
								dprint(dara.DEBUG, func() { l.Println("Replay Consumed", len(events), "events") })
								for _, e := range events {
									dprint(dara.DEBUG, func(){ l.Println(common.ConciseEventString(&e))})
									same_event := CompareEvents(e, schedule.LogEvents[i])
									if !same_event {
										dprint(dara.WARN, func() {
											l.Println("Replayed 2 different events", common.ConciseEventString(&e), common.ConciseEventString(&schedule.LogEvents[i]))
										})
									}
									i++
								}
								procStatus[currentProc] = DEAD
								break
							}
							atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[currentDaraProc].Lock))), dara.UNLOCKED)
						}
					}
					time.Sleep(time.Second)
				}
			}
		}
	}
	dprint(dara.INFO, func() { l.Println("Replay is over") })
}

// extractDataFromProc consumes events and coverage information and then check properties
// Context, schedule, and the index are modified by this function
func extractDataFromProc(ProcID int, index *int, schedule *dara.Schedule, context *map[string]interface{}) (*[]dara.Event, *explorer.CovStats, bool) {
	events := ConsumeAndPrint(ProcID, context)
	coverage := ConsumeCoverage(ProcID)
	dprint(dara.DEBUG, func() { l.Println("Consumed ", len(coverage), "blocks") })
	result, propevents, err := checkProperties(*context)
	if err != nil {
		dprint(dara.INFO, func() { l.Println(err) })
	}
	if !result {
		dprint(dara.INFO, func() { l.Println("Property check failed with", len(*propevents), "failures") })
	}
	schedule.PropEvents = append(schedule.PropEvents, dara.CreatePropCheckEvent(*propevents, *index + len(events) - 1))
	schedule.LogEvents = append(schedule.LogEvents, events...)
	coverageEvent := dara.CoverageEvent{CoverageInfo:coverage, EventIndex: *index + len(events) - 1}
	schedule.CovEvents = append(schedule.CovEvents, coverageEvent)
	hasBug := !result
	for _, e := range events {
		if e.Type == dara.CRASH_EVENT {
			hasBug = true
			break
		}
	}
	*index += len(events)
	return &events, &coverage, hasBug
}

// recordSchedule records a new execution
func recordSchedule() {
	LastProc = 0
	var i int
	// Context for property checking
	context := make(map[string]interface{})
	for i < RECORDLEN {
		if CheckAllProcsDead() {
			// If all processes have completed then break the loop and move out.
			break
		}
		ProcID := roundRobin()
		dprint(dara.WARN, func() {l.Println("Chosen process for running", ProcID)} )
		//dprint(dara.INFO, func() { l.Println("Chosen process for running is", ProcID) })
		if procchan[ProcID].Run == -1 {
			// Initialize the local scheduler to record events
			procchan[ProcID].Run = -3
			dprint(dara.INFO, func() { l.Println("Setting up recording on Process", ProcID) })
			dprint(dara.INFO, func() { l.Println("Linux PID for this process is", procchan[ProcID].PID)})
			atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED)
			lockStatus[ProcID] = false
			forward() // Give the local scheduler a chance to grab the lock
		}
		// Loop until we get some recorded events
		for {
			if lockStatus[ProcID] || atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED, dara.LOCKED) {
				if linuxpids[ProcID] == 0 && procchan[ProcID].PID != 0{
					linuxpids[ProcID] = procchan[ProcID].PID
					dprint(dara.INFO, func() { l.Println("Linux PID for this process is", procchan[ProcID].PID)})
				}
				// We own the lock now if we didn't before.
				lockStatus[ProcID] = true
				//dprint(dara.DEBUG, func() { l.Println("Obtained Lock with run value", procchan[ProcID].Run, "on process", ProcID) })
				if procchan[ProcID].Run == -3 {
					// The local scheduler hasn't run anything yet.
					atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED)
					// Release the lock
					lockStatus[ProcID] = false
					continue
				}
				if procchan[ProcID].Run == -100 { 
					// Process has finished
					// Ignore the return values since we really don't need them in record.
					// Return value of extractDataFromProc are only useful during exploration
					_, _, _ = extractDataFromProc(ProcID, &i, &schedule, &context)
				}
				if procchan[ProcID].Run == -6 {
					//Process is blocked on network, hopefully it is unblocked after a full cycle of process scheduling!
					// If this is the first time we chose this Process to run after getting blocked then we need
					// to consume the events and the coverage
					if procchan[ProcID].LogIndex > 0 {
						_, _, _ = extractDataFromProc(ProcID, &i, &schedule, &context)
					}
					// Continue to hold the lock until the process has become unblocked
					break
				}
				if procchan[ProcID].Run == -7 {
					// Process has woken up from the Network poll and wants the lock back.
					dprint(dara.DEBUG, func() { l.Println("Process", ProcID, " has become unblocked from network wait") })
					atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED)
					procchan[ProcID].Run = -3
					lockStatus[ProcID] = false
					forward() //Give the process a chance to grab the lock
					continue
				}
				if procchan[ProcID].Run >= 0 {
					// Process scheduled a goroutine which means we have events to add to the schedule.
					dprint(dara.DEBUG, func() { l.Printf("Procchan Run status inside is %d for Process %d\n", procchan[ProcID].Run, ProcID) })
					dprint(dara.DEBUG, func() { l.Printf("Recording Event on Process/Node %d\n", ProcID) })
					_, _, _ = extractDataFromProc(ProcID, &i, &schedule, &context)
					if i >= RECORDLEN-*procs {
						//mark the processes for death
						dprint(dara.DEBUG, func() { l.Printf("Ending Execution!") })
						procchan[ProcID].Run = -4 //mark process for death
					} else {
						//Reset the local scheduler to continue with its wizardry.
						procchan[ProcID].Run = -3
					}
					// We got some events so now we should run some other process.
					break
				}
				if CheckAllGoRoutinesDead(ProcID) {
					procStatus[ProcID] = DEAD
					dprint(dara.INFO, func() { l.Println("Process ", ProcID, " has finished")})
					break
				} else {
					dprint(dara.INFO, func() { l.Println("Run value for process ", ProcID, " is ", procchan[ProcID].Run)})
				}
			}
		}
	}
	// Create the schedule file and encode/store the captured schedule
	f, err := os.Create(*sched_file)
	if err != nil {
		l.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "\t")
	err = enc.Encode(schedule)
	if err != nil {
		l.Fatal(err)
	}
}

// dara_rpc_client launches the rpc client for communicating with the overlord
func dara_rpc_client(command chan string, response chan bool) {
    client, err := rpc.Dial("tcp", OVERLORD_RPC_SERVER)
    if err != nil {
        l.Fatal(err)
    }

    var isFinished bool

    for ;!isFinished;{
        select {
            case op := <-command:
                switch op {
					case "finish":
						dprint(dara.INFO, func(){l.Println("Contacting overlord to finish execution")})
                        var reply bool
                        err = client.Call("DaraRpcServer.FinishExecution", 0, &reply)
                        if err != nil {
							l.Println(err)
							response <- false
                        }
						isFinished = true
						response <- true
                    case "kill":
                        var reply bool
                        err = client.Call("DaraRpcServer.KillExecution", 0, &reply)
                        if err != nil {
							l.Println(err)
							response <- false
						}
						response <- true
					case "restart":
						var reply bool
                        err = client.Call("DaraRpcServer.RestartExecution", 0, &reply)
                        if err != nil {
							l.Println(err)
							response <- false
						}
						response <- true
                }
        }
    }
}

// explore explores the schedule + coverage space
func exploreSchedule() {
	schedule := decodeInputSchedule()
	command_chan := make(chan string)
	response_chan := make(chan bool)
	go dara_rpc_client(command_chan, response_chan)
	explore_unit := initExplorer(schedule)
	explore_unit.SetMaxDepth(MAX_EXPLORATION_DEPTH)
	dprint(dara.INFO, func() { l.Println("Dara the Explorer begins")})
	current_run := 0
	restart := false
	start := true
	var all_times []float64
	var run_times []float64
	var firstBugRun int
	var numbuggyPaths int
	var bugruns []int
	for current_run < MAX_EXPLORE_RUNS {
		l.Println("Exploring path #", current_run + 1)
		isblocked := make(map[int]bool)
		if restart {
			// Reset shared memory and all the status
			for i := 0; i <= *procs; i++ {
				dprint(dara.DEBUG, func() {l.Println("Re-Initializing", i) })
				procchan[i].Lock = dara.LOCKED
				procchan[i].SyscallLock = dara.UNLOCKED
				procchan[i].Run = -1
				procchan[i].Syscall = -1
				procchan[i].CoverageIndex = 0
				procchan[i].RunningRoutine = dara.RoutineInfo{}
				for j := 0; j < len(procchan[i].Routines); j++ {
					procchan[i].Routines[j] = dara.RoutineInfo{}
				}
			}
			for i := 1; i <= *procs; i++ {
				procStatus[i] = ALIVE
				lockStatus[i] = true // By default, global scheduler owns the locks on every local scheduler
				linuxpids[i] = uint64(0)
				covStats := make(explorer.CovStats)
				allCoverage[i] = &covStats 
				clocks[i] = explorer.NewClock()
				isblocked[i] = false
			}
			command_chan <- "restart"
			response := <- response_chan
			if response {
				dprint(dara.INFO, func() {l.Println("Overlord restarted execution successfully")})
			} else {
				dprint(dara.INFO, func() {l.Println("Overlord didn't restart execution successfully")})
			}
		}
		if restart || start {
			for i := 1; i <= *procs; i++ {
				isblocked[i] = false
			}
			// Give time for the program to start/restart
			time.Sleep(5 * time.Second)
			start = false
			//time.Sleep(5 * time.Millisecond)
		}
		runStart := time.Now()
		// Initialize variables for this run
		var event_index int
		var current_schedule dara.Schedule // current schedule
		current_context := make(map[string]interface{}) // current context for property checking

		// Reset the linux pids for each process!
		for i := 1; i <= *procs; i++ {
			linuxpids[i] = 0
		}

		// The choice of goroutine to be run is broken down into 2 parts
		// Part 1: Selecting the Process
		// Part 2: Selecting the goroutine on the process
		actionComplete := true
		actionInstalled := false
		actionSelected := false
		var ProcID int
		var events *[]dara.Event
		var coverage *explorer.CovStats
		var next_routine *explorer.ProcThread
		var next_timer *explorer.ProcTimer
		var action explorer.Action
		var numActions int
		var bugFound bool
		dprint(dara.INFO, func() {l.Println("Before inner loop")})
		for {
			// Have the explorer choose the process
			allBlocked := true
			var hasBug bool
			for _, v := range isblocked {
				allBlocked = allBlocked && v
			}
			if actionComplete && !allBlocked{
				// Get New action now!
				numActions += 1
				threads := getAllDaraProcs()
				start := time.Now()
				action = explore_unit.GetNextAction(threads, events, coverage, &clocks, isblocked)
				end := time.Now().Sub(start)
				all_times = append(all_times, end.Seconds())
				// If action is restart then kill current execution and restart
				if action.Type == explorer.RESTART {
					command_chan <- "kill"
					response := <-response_chan
					if response {
						dprint(dara.INFO, func() {l.Println("Overlord finished execution successfully")})
					} else {
						dprint(dara.INFO, func() {l.Println("Overlord didn't finish execution successfully")})
					}
					restart = true
					// Update the run counter and break away to the next iteration
					current_run += 1
					break
				} else if action.Type == explorer.SCHEDULE {
					if ptr, ok := action.Arg.(*explorer.ProcThread); ok {
						next_routine = ptr
						ProcID = next_routine.ProcID
						dprint(dara.INFO, func() {l.Println("Goroutine chosen to be scheduled is", next_routine.Thread.Gid)})
					} else {
						dprint(dara.FATAL, func() {l.Println("Failed to parse explorer action")} )
					}
				} else if action.Type == explorer.TIMER {
					if ptr, ok := action.Arg.(*explorer.ProcTimer); ok {
						next_timer = ptr
					} else {
						dprint(dara.FATAL, func() {l.Println("Failed to parse explorer action")} )
					}
				}
				
				actionSelected = true
				actionComplete = false
			} else if allBlocked {
				actionSelected = false
			}
			// Loop until chosen action is completed
			if lockStatus[ProcID] || atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&procchan[ProcID].Lock)), dara.UNLOCKED, dara.LOCKED) {
				lockStatus[ProcID] = true
				if linuxpids[ProcID] == 0 && procchan[ProcID].PID !=0 {
					linuxpids[ProcID] = procchan[ProcID].PID
				}
				if procchan[ProcID].Run == -100 {
					// The process has ended 
					// Extract data and update the variables from this run!
					// We don't need the events and coverage anymore for this event
					// They are already in the schedule
					if actionInstalled {
						events, coverage, hasBug = extractDataFromProc(ProcID, &event_index, &current_schedule, &current_context)
						explorer.CoverageUnion(allCoverage[ProcID], coverage)
						explore_unit.UpdateActionCoverage(events, coverage)
						dprint(dara.INFO, func() {l.Println("Process has finished")})
						// Check for crash events and property failures. Save the current schedule if there are any such failures
						bugFound = bugFound || hasBug
						// Mark the process as DEAD
						procStatus[ProcID] = DEAD
						actionComplete = true
						actionInstalled = false
					}
					// We shouldn't be in this state if action was installed
				} else if procchan[ProcID].Run == -5 {
					// The process is executing some previously chosen instruction
					// It is unlikely we ever get the lock back in this state
					// Regardless, we should unlock and try again.
					atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED)
					lockStatus[ProcID] = false
					continue
				} else if procchan[ProcID].Run == -1 {
					// Local Runtime is either waiting for instructions about the next choice to be made
					// or it has completed our action
					if actionInstalled {
						// Action has now been completed. Woohoo!
						actionComplete = true
						actionInstalled = false
						events, coverage, hasBug = extractDataFromProc(ProcID, &event_index, &current_schedule, &current_context)
						// Update the full coverage
						explorer.CoverageUnion(allCoverage[ProcID], coverage)
						explore_unit.UpdateActionCoverage(events, coverage)
						// Check for crash events and property failures. Save the current schedule if there are any such failures
						bugFound = bugFound || hasBug
						// Keep holding the lock until we need to schedule a new action for this process. Don't release the lock.
					} else {
						// Need to install the action
						actionInstalled = true
						dprint(dara.INFO, func() {l.Println("Installing a new action")})
						procchan[ProcID].Run = -5
						if action.Type == explorer.SCHEDULE {
							// Either the action is to schedule a goroutine
							procchan[ProcID].RunningRoutine = next_routine.Thread
						} else if action.Type == explorer.TIMER {
							// Or the action is to fire off a timer
							// Need to add the timer firing event to the schedule manually.
							procchan[ProcID].RunningRoutine.Gid = int(next_timer.ID) // timerID
							procchan[ProcID].RunningRoutine.RoutineCount = -5 // Special Code
						}

						// Release the lock so that the installed action can be completed.
						lockStatus[ProcID] = false
						atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED)
					}
				} else if procchan[ProcID].Run == -6 {
					// This process is now blocked polling on network. Need to switch different processes!
					isblocked[ProcID] = true
					dprint(dara.DEBUG, func() {l.Println("Process", ProcID, "blocked polling on network")})
					if actionInstalled {
						dprint(dara.INFO, func() { l.Println("action was installed")})
						// Action has now been completed. Woohoo!
						actionComplete = true
						actionInstalled = false
						events, coverage, hasBug = extractDataFromProc(ProcID, &event_index, &current_schedule, &current_context)
						// Update the full coverage
						explorer.CoverageUnion(allCoverage[ProcID], coverage)
						explore_unit.UpdateActionCoverage(events, coverage)
						// Check for crash events and property failures. Save the current schedule if there are any such failures
						bugFound = bugFound || hasBug
					}
				} else if procchan[ProcID].Run == -7 {
					// This process is now available to be run woohoo!
					isblocked[ProcID] = false
					dprint(dara.INFO, func() {l.Println("Process", ProcID, "unblocked from network poll")})
					atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED)
					lockStatus[ProcID] = false
					if actionSelected {
						// Possibly schedule that action?
					}
				}
			}
			if CheckAllProcsDead() {
				// All the processes are dead so we can move on to the next run/finish exploration
				current_run += 1
				explore_unit.RestartExploration()
				restart = true
				break
			}
		}
		runend := time.Now().Sub(runStart)
		run_times = append(run_times, runend.Seconds())
		l.Println("Total Number of actions:", numActions)
		if bugFound {
			numbuggyPaths++
			if firstBugRun == 0 {
				firstBugRun = current_run
			}
			bugruns = append(bugruns, current_run)
		}
	}
	// Notify the overlord that we are done with exploration
	l.Println("Total number of buggy paths", numbuggyPaths)
	l.Println("First bug run:", firstBugRun)
	command_chan <- "finish"
	response := <-response_chan
	if response {
		dprint(dara.INFO, func() {l.Println("Overlord finished execution successfully")})
	} else {
		dprint(dara.INFO, func() {l.Println("Overlord didn't finish execution successfully")})
	}
	dprint(dara.INFO, func() { l.Println("Exploration ended") })
	err := explore_unit.SaveVisitedSchedules()
	if err != nil {
		dprint(dara.FATAL, func() { l.Fatal(err) })
	}
	file, err := os.Create("action_times-" + *strategy + ".csv")
	if err != nil {
		dprint(dara.FATAL, func() {l.Fatal(err)})
	}
	defer file.Close()
	for _, t := range all_times {
		file.WriteString(fmt.Sprintf("%f\n", t))
	}
	rfile, err := os.Create("run_times-" + *strategy + ".csv")
	if err != nil {
		dprint(dara.FATAL, func() {l.Fatal(err)})
	}
	defer rfile.Close()
	for _, t := range run_times {
		rfile.WriteString(fmt.Sprintf("%f\n", t))
	}
	err = explore_unit.SaveStateSpaceResults("states-" + *strategy + ".csv")
	if err != nil {
		dprint(dara.FATAL, func() {l.Fatal(err)})
	}
	bugrunfile, err := os.Create("bugruns-" + *strategy + ".csv")
	if err != nil {
		dprint(dara.FATAL, func(){l.Fatal(err)})
	}
	defer bugrunfile.Close()
	for _, v := range bugruns {
		bugrunfile.WriteString(fmt.Sprintf("%d\n", v))
	}
}

// initGlobalScheduler initializes the global scheduler
// This includes setting up the shared memory, initialising the global variables
func initGlobalScheduler() {
	//Set up logger
	l = log.New(os.Stdout, "[Scheduler]", log.Lshortfile)
	checkargs()
	var err2 error
	prop_file := os.Getenv("PROP_FILE")
	if prop_file != "" {
		dprint(dara.INFO, func() { l.Println("Using property file", prop_file) })
		checker, err2 = propchecker.NewChecker(prop_file)
		if err2 != nil {
			l.Fatal(err2)
		}
	}
	p, err = runtime.Mmap(nil,
		dara.CHANNELS*dara.DARAPROCSIZE,
		dara.PROT_READ|dara.PROT_WRITE, dara.MAP_SHARED, dara.DARAFD, 0)

	if err != 0 {
		log.Println(err)
		l.Fatal(err)
	}

	//Map control struct into shared memory
	procchan = (*[dara.CHANNELS]dara.DaraProc)(p)
	//rand.Seed(int64(time.Now().Nanosecond()))
	//var count int
    dprint(dara.INFO, func() { l.Println("Size of DaraProc struct is", unsafe.Sizeof(procchan[0]))})
	for i := 0; i <= *procs; i++ {
        dprint(dara.DEBUG, func() {l.Println("Initializing", i) })
		procchan[i].Lock = dara.LOCKED
		procchan[i].SyscallLock = dara.UNLOCKED
		procchan[i].Run = -1
		procchan[i].Syscall = -1
        procchan[i].CoverageIndex = 0
	}
	procStatus = make(map[int]ProcStatus)
	lockStatus = make(map[int]bool)
	linuxpids = make(map[int]uint64)
	allCoverage = make(map[int]*explorer.CovStats)
	clocks = make(map[int]*explorer.VirtualClock)
	for i := 1; i <= *procs; i++ {
		procStatus[i] = ALIVE
		lockStatus[i] = true // By default, global scheduler owns the locks on every local scheduler
		linuxpids[i] = uint64(0)
		covStats := make(explorer.CovStats)
		allCoverage[i] = &covStats 
		clocks[i] = explorer.NewClock()
	}
	dprint(dara.INFO, func() { l.Println("Starting the Scheduler") })

}

func main() {
	initGlobalScheduler()
	if *replay {
		dprint(dara.INFO, func() { l.Println("Started replaying") })
		replaySchedule()
		dprint(dara.INFO, func() { l.Println("Finished replaying") })
	} else if *record {
		dprint(dara.INFO, func() { l.Println("Started recording") })
		recordSchedule()
		dprint(dara.INFO, func() { l.Println("Finished recording") })
	} else if *explore {
		dprint(dara.INFO, func() { l.Println("Started exploring")})
		exploreSchedule()
		dprint(dara.INFO, func() { l.Println("Finished exploring") })
	}
	dprint(dara.INFO, func() { l.Println("Exiting scheduler") })
}
