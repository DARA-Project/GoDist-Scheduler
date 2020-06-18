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
)

const (
	//The length of the schedule. When recording the system will
	//execute up to dara.SCHEDLEN. The same is true on replay
	RECORDLEN             = 10000
	EXPLORELEN            = 100
	EXPLORATION_LOG_FILE  = "visitedLog.txt"
	MAX_EXPLORATION_DEPTH = 10
    // TODO: Make this an option
    OVERLORD_RPC_SERVER = "0.0.0.0:45000"
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

	// Map that maps the Dara Process ID to its status
	procStatus map[int]ProcStatus
	// Map that maps the Dara Process ID to whether we own the procchan lock for that process
	lockStatus map[int]bool
	// Map that maps Dara Process ID to linux PID
	linuxpids map[int]uint64
)

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
	} else {
		l.Print("Replaying")
	}
	level := os.Getenv("DARA_LOG_LEVEL")
	i, err := strconv.Atoi(level)
	if err != nil {
		l.Fatal("Invalid log level specified")
	}
	LogLevel = int(i)
	return
}

func forward() {
	time.Sleep(1 * time.Millisecond)
}

// level_print is a log level print function for the Global Scheduler
func level_print(level int, pfunc func()) {
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
		level_print(dara.DEBUG, func() {l.Println("Checking status of", LastProc, "which is", procStatus[LastProc])})
		if procStatus[LastProc] == ALIVE {
			level_print(dara.DEBUG, func() {l.Println("Next process should be", LastProc)})
			flag = true
		}
	}
	return LastProc
}

// getSchedulingEvents returns all the scheduling events in a schedule
func getSchedulingEvents(sched dara.Schedule) map[int][]dara.Event {
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
func ConsumeCoverage(ProcID int) map[string]uint64 {
    coverage := make(map[string]uint64)
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
	level_print(dara.DEBUG, func() {l.Println("Consumed", len(cl), "events")})
	return cl
}

// ConsumeLog consumes all the events from the shared memory and produces
// a list of events for further use
func ConsumeLog(ProcID int, context *map[string]interface{}) []dara.Event {
	level_print(dara.DEBUG, func() { l.Println("Current log event index is", procchan[ProcID].LogIndex, "for Process", ProcID) })
	logsize := procchan[ProcID].LogIndex
	level_print(dara.DEBUG, func() { l.Println("Current log event index is", logsize, "for Process", ProcID) })
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
		//(*e).Msg = (*ee).EM //TODO complete messages
		level_print(dara.DEBUG, func() { l.Println(common.ConciseEventString(e)) })
		if e.Type == dara.THREAD_EVENT {
			level_print(dara.DEBUG, func() { l.Println("Thread creation noted with ID", e.G.Gid, "at function", string(e.G.FuncInfo[:64])) })
		}
		if e.Type == dara.SCHED_EVENT {
			level_print(dara.INFO, func() { l.Println("Scheduling event for Process", ProcID, "and goroutine", common.RoutineInfoString(&e.G))})
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
	level_print(dara.DEBUG, func() { l.Printf("Checking if all goroutines are dead yet for Process %d\n", ProcID) })
	//l.Printf("Num routines are %d", len(procchan[ProcID].Routines))
	for _, routine := range procchan[ProcID].Routines {
		// Only check for dead goroutines for real go routines
		if routine.Gid != 0 {
			status := dara.GetDaraProcStatus(routine.Status)
			level_print(dara.DEBUG, func() { l.Printf("Status of GoRoutine %d is %s\n", routine.Gid, dara.GStatusStrings[status]) })
			allDead = allDead && (status == dara.Dead)
            if status != dara.Dead {
                //level_print(dara.INFO, func() { l.Printf("GoRoutine %d Proc %d is not dead yet\n", routine.Gid, ProcID)})
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
			level_print(dara.DEBUG, func() { l.Printf("Proc[%d]Routine[%d].Info = %s", event.P, j, common.RoutineInfoString(&info)) })
			runnable = append(runnable, j)
		}
	}
	return runnable
}

// Is_event_replayable checks if an event is replayable if the goid to be
// scheduled for that event is one of the runnable routines.
func Is_event_replayable(runnable []int, Gid int) bool {
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

// Initializes the explorer
// TODO: Add a seed schedule to the explorer
func explore_init() *explorer.Explorer {
	e, err := explorer.MountExplorer(EXPLORATION_LOG_FILE, explorer.RANDOM)
	if err != nil {
		l.Fatal(err)
	}
	e.SetMaxDepth(MAX_EXPLORATION_DEPTH)
	return e
}

// getDaraProcs returns the list of goroutines that explorer can choose from for scheduling
// TODO: Maybe we should filter out dead goroutines here
func getDaraProcs() []explorer.ProcThread {
	threads := []explorer.ProcThread{}
	for i := 1; i <= *procs; i++ {
		for _, routine := range procchan[i].Routines {
			if dara.GetDaraProcStatus(routine.Status) != dara.Idle {
				t := explorer.ProcThread{ProcID: i, Thread: routine}
				threads = append(threads, t)
			}
		}
	}
	return threads
}

// preload_replay_schedule populates the shared memory with a pre-chosen schedule
func preload_replay_schedule(proc_schedules map[int][]dara.Event) {
	for procID, events := range proc_schedules {
		for i, e := range events {
			procchan[procID].Log[i].Type = e.Type
			procchan[procID].Log[i].P = e.P
			procchan[procID].Log[i].G = e.G
		}
		procchan[procID].LogIndex = len(events)
	}
}

// check_properties checks the user-defined properties under a given context
// that is collected using DaraLog calls in the local runtimes
func check_properties(context map[string]interface{}) (bool, *[]dara.FailedPropertyEvent, error) {
	if checker != nil {
		return checker.Check(context)
	}
	return true, &[]dara.FailedPropertyEvent{}, nil
}

// replay_sched replays a previously recorded/explored execution
func replay_sched() {
	var i int
	f, err := os.Open(*sched_file)
	if err != nil {
		l.Fatal(err)
	}
	dec := json.NewDecoder(f)
	err = dec.Decode(&schedule)
	if err != nil {
		l.Fatal(err)
	}
	context := make(map[string]interface{})
	level_print(dara.DEBUG, func() { l.Println("Num Events : ", len(schedule.LogEvents)) })
	level_print(dara.DEBUG, func() { l.Println("Replaying the following schedule : ", common.ConciseScheduleString(&schedule.LogEvents)) })
	fast_replay := os.Getenv("FAST_REPLAY")
	if fast_replay == "true" {
		proc_schedule_map := getSchedulingEvents(schedule)
		preload_replay_schedule(proc_schedule_map)
		for procID := range procchan {
			if procchan[procID].Run == -1 && procchan[procID].LogIndex == 0 {
				procchan[procID].Run = -100
			} else {
				level_print(dara.INFO, func() { l.Println("Enabling runtime for proc", procID) })
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
						//level_print(dara.INFO, func() {l.Println("Not over for Proc", procID, "with value", procchan[procID].Run)})
					}
					atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[procID].Lock))), dara.UNLOCKED)
				} else {
					all_procs_done = false
				}
			}
		}
		level_print(dara.INFO, func() { l.Println("Fast Replay done") })
	} else {
		for i < len(schedule.LogEvents) {
			level_print(dara.DEBUG, func(){l.Println("Replaying event index", i)})
			if CheckAllProcsDead() {
				level_print(dara.INFO, func() {l.Println("All processes dead but", len(schedule.LogEvents) - i, "events need to be replayed")})
				break
			}
			if lockStatus[schedule.LogEvents[i].P] || atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[schedule.LogEvents[i].P].Lock))), dara.UNLOCKED, dara.LOCKED) {
				if procchan[schedule.LogEvents[i].P].Run == -1 { //check predicates on goroutines + schedule

					//move forward
					forward()

					level_print(dara.DEBUG, func() { l.Println("Scheduling Event") })
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
						can_run := Is_event_replayable(runnable, schedule.LogEvents[i].G.Gid)
						if !can_run {
							if string(bytes.Trim(schedule.LogEvents[i].G.FuncInfo[:64], "\x00")[:]) != "runtime.timerproc" {
								level_print(dara.INFO, func() { l.Println("G info was ", string(schedule.LogEvents[i].G.FuncInfo[:64])) })
								level_print(dara.FATAL, func() { l.Fatal("Can't run event ", i, ": ", common.ConciseEventString(&schedule.LogEvents[i])) })
							} else {
								//Trying to schedule timer thread. Let's just increment the event counter
								level_print(dara.INFO, func() { l.Println("G info was ", string(schedule.LogEvents[i].G.FuncInfo[:64])) })
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
					level_print(dara.INFO, func() {
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
								level_print(dara.DEBUG, func() { l.Println("Replay Consumed ", len(events), "events") })
                                level_print(dara.DEBUG, func() { l.Println("Replay Consumed ", len(coverage), "blocks") })
								for _, e := range events {
									level_print(dara.DEBUG, func(){ l.Println(common.ConciseEventString(&e))})
									same_event := CompareEvents(e, schedule.LogEvents[i])
									if !same_event {
										level_print(dara.WARN, func() {
											l.Println("Replayed 2 different events", common.ConciseEventString(&e), common.ConciseEventString(&schedule.LogEvents[i]))
										})
									} else {
										level_print(dara.DEBUG, func() { l.Println("Replayed ", common.ConciseEventString(&schedule.LogEvents[i])) })
									}
									i++
								}
								level_print(dara.DEBUG, func() { l.Println("Replay : At event", i) })
								if i >= len(schedule.LogEvents) {
									level_print(dara.DEBUG, func() { l.Printf("Informing the local scheduler end of replay") })
									procchan[currentDaraProc].Run = -4
									break
								}
								break
							} else if procchan[schedule.LogEvents[i].P].Run == -100 {
								// This means that the local runtime finished and that we can move on
								currentProc := schedule.LogEvents[i].P
								events := ConsumeAndPrint(currentDaraProc, &context)
                                coverage := ConsumeCoverage(currentDaraProc)
                                level_print(dara.DEBUG, func() { l.Println("Replay Consumed ", len(coverage), "blocks") })
								level_print(dara.DEBUG, func() { l.Println("Replay Consumed", len(events), "events") })
								for _, e := range events {
									level_print(dara.DEBUG, func(){ l.Println(common.ConciseEventString(&e))})
									same_event := CompareEvents(e, schedule.LogEvents[i])
									if !same_event {
										level_print(dara.WARN, func() {
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
	level_print(dara.INFO, func() { l.Println("Replay is over") })
}

// record_sched records a new execution
func record_sched() {
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
		level_print(dara.INFO, func() { l.Println("Chosen process for running is", ProcID) })
		if procchan[ProcID].Run == -1 {
			// Initialize the local scheduler to record events
			procchan[ProcID].Run = -3
			level_print(dara.INFO, func() { l.Println("Setting up recording on Process", ProcID) })
			level_print(dara.INFO, func() { l.Println("Linux PID for this process is", procchan[ProcID].PID)})
			atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED)
			lockStatus[ProcID] = false
			forward() // Give the local scheduler a chance to grab the lock
		}
		// Loop until we get some recorded events
		for {
			if lockStatus[ProcID] || atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED, dara.LOCKED) {
				if linuxpids[ProcID] == 0 && procchan[ProcID].PID != 0{
					linuxpids[ProcID] = procchan[ProcID].PID
					level_print(dara.INFO, func() { l.Println("Linux PID for this process is", procchan[ProcID].PID)})
				}
				// We own the lock now if we didn't before.
				lockStatus[ProcID] = true
				level_print(dara.DEBUG, func() { l.Println("Obtained Lock with run value", procchan[ProcID].Run, "on process", ProcID) })
				if procchan[ProcID].Run == -3 {
					// The local scheduler hasn't run anything yet.
					atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED)
					// Release the lock
					lockStatus[ProcID] = false
					continue
				}
				if procchan[ProcID].Run == -100 { 
					// Process has finished
					events := ConsumeAndPrint(ProcID, &context)
					coverage := ConsumeCoverage(ProcID)
					level_print(dara.DEBUG, func() { l.Println("Record Consumed ", len(coverage), "blocks") })
					result, propevents, err := check_properties(context)
					if err != nil {
						level_print(dara.INFO, func() { l.Println(err) })
					}
					if !result {
						level_print(dara.INFO, func() { l.Println("Property check failed with", len(*propevents), "failures") })
					}
					schedule.PropEvents = append(schedule.PropEvents, dara.CreatePropCheckEvent(*propevents, i + len(events) - 1))
					schedule.LogEvents = append(schedule.LogEvents, events...)
					coverageEvent := dara.CoverageEvent{CoverageInfo:coverage, EventIndex: i + len(events) - 1}
					schedule.CovEvents = append(schedule.CovEvents, coverageEvent)
				}
				if procchan[ProcID].Run == -6 {
					//Process is blocked on network, hopefully it is unblocked after a full cycle of process scheduling!
					// If this is the first time we chose this Process to run after getting blocked then we need
					// to consume the events and the coverage
					if procchan[ProcID].LogIndex > 0 {
						events := ConsumeAndPrint(ProcID, &context)
						coverage := ConsumeCoverage(ProcID)
						result, propevents, err := check_properties(context)
						if err != nil {
							level_print(dara.INFO, func() {l.Println(err)})
						}
						if !result {
							level_print(dara.INFO, func() { l.Println("Property check failed with", len(*propevents), "failures") })
						}
						schedule.PropEvents = append(schedule.PropEvents, dara.CreatePropCheckEvent(*propevents, i + len(events) - 1))
						schedule.LogEvents = append(schedule.LogEvents, events...)
						coverageEvent := dara.CoverageEvent{CoverageInfo: coverage, EventIndex: i + len(events) - 1}
						schedule.CovEvents = append(schedule.CovEvents, coverageEvent)
					}
					// Continue to hold the lock until the process has become unblocked
					break
				}
				if procchan[ProcID].Run == -7 {
					// Process has woken up from the Network poll and wants the lock back.
					level_print(dara.DEBUG, func() { l.Println("Process", ProcID, " has become unblocked from network wait") })
					atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED)
					procchan[ProcID].Run = -3
					lockStatus[ProcID] = false
					continue
				}
				if procchan[ProcID].Run > 0 {
					// Process scheduled a goroutine which means we have events to add to the schedule.
					level_print(dara.DEBUG, func() { l.Printf("Procchan Run status inside is %d for Process %d\n", procchan[ProcID].Run, ProcID) })
					level_print(dara.DEBUG, func() { l.Printf("Recording Event on Process/Node %d\n", ProcID) })
					events := ConsumeAndPrint(ProcID, &context)
					coverage := ConsumeCoverage(ProcID)
					level_print(dara.DEBUG, func() { l.Println("Record Consumed ", len(coverage), "blocks") })
					result, propevents, err := check_properties(context)
					if err != nil {
						level_print(dara.INFO, func() { l.Println(err) })
					}
					if !result {
						level_print(dara.INFO, func() { l.Println("Property check failed with", len(*propevents), "failures") })
					}
					schedule.PropEvents = append(schedule.PropEvents, dara.CreatePropCheckEvent(*propevents, i + len(events) - 1))
					schedule.LogEvents = append(schedule.LogEvents, events...)							
					coverageEvent := dara.CoverageEvent{CoverageInfo:coverage, EventIndex: i + len(events) - 1}
					schedule.CovEvents = append(schedule.CovEvents, coverageEvent)
					if i >= RECORDLEN-*procs {
						//mark the processes for death
						level_print(dara.DEBUG, func() { l.Printf("Ending Execution!") })
						procchan[ProcID].Run = -4 //mark process for death
					} else {
						//Reset the local scheduler to continue with its wizardry.
						procchan[ProcID].Run = -3
					}

					level_print(dara.DEBUG, func() { l.Printf("Found %d log events", len(events)) })
					i += len(events)
					// We got some events so now we should run some other process.
					break
				}
				if CheckAllGoRoutinesDead(ProcID) {
					procStatus[ProcID] = DEAD
					level_print(dara.INFO, func() { l.Println("Process ", ProcID, " has finished")})
					break
				}
			}
		}
	}
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
func dara_rpc_client(command chan string) {
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
                        var reply bool
                        err = client.Call("DaraRpcServer.FinishExecution", 0, &reply)
                        if err != nil {
                            l.Fatal(err)
                        }
                        isFinished = true
                    case "kill":
                        var reply bool
                        err = client.Call("DaraRpcServer.KillExecution", 0, &reply)
                        if err != nil {
                            l.Fatal(err)
                        }
                }
        }
    }
}

// explore_sched explores the schedule space
func explore_sched() {
    command_chan := make(chan string)
    go dara_rpc_client(command_chan)
	explore_unit := explore_init()
	level_print(dara.INFO, func() { l.Println("Dara the Explorer begins") })
	LastProc = 0
	explore_end := false
	context := make(map[string]interface{})
	var i int
	for i < EXPLORELEN {
		ProcID := roundRobin()
		if explore_end {
			break
		}
		if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED, dara.LOCKED) {
			if procchan[ProcID].Run == -100 {
				events := ConsumeAndPrint(ProcID, &context)
                coverage := ConsumeCoverage(ProcID)
                level_print(dara.DEBUG, func() { l.Println("Explore Consumed ", len(coverage), "blocks") })
				schedule.LogEvents = append(schedule.LogEvents, events...)
				coverageEvent := dara.CoverageEvent{CoverageInfo:coverage, EventIndex: i + len(events) - 1}
				schedule.CovEvents = append(schedule.CovEvents, coverageEvent)
				i += len(events)
				// Check if one of them is a crash or end event. If so, exploration should be over.
				// As they both are ending events, we only need to check the last event in the schedule
				for _, e := range events {
					if CheckEndOrCrash(e) {
						level_print(dara.INFO, func() { l.Println("End or Crash discovered") })
						explore_end = true
						continue
					}
				}
			}
			if procchan[ProcID].Run == -1 { //TODO check predicates on goroutines + schedule
				forward()
				procchan[ProcID].Run = -5

				atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED)
				flag := true
				for flag {
					if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED, dara.LOCKED) {
						//l.Printf("procchan[schedule[%d]].Run = %d",i,procchan[schedule[i]].Run)
						if procchan[ProcID].Run == -1 {
							events := ConsumeAndPrint(ProcID, &context)
                            coverage := ConsumeCoverage(ProcID)
                            level_print(dara.DEBUG, func() { l.Println("Explore Consumed ", len(coverage), "blocks") })
							schedule.LogEvents = append(schedule.LogEvents, events...)
							coverageEvent := dara.CoverageEvent{CoverageInfo:coverage, EventIndex: i + len(events) - 1}
							schedule.CovEvents = append(schedule.CovEvents, coverageEvent)
							i += len(events)
							// Check if one of them is a crash or end event. If so, exploration should be over.
							// As they both are ending events, we only need to check the last event in the schedule
							for _, e := range events {
								if CheckEndOrCrash(e) {
									level_print(dara.INFO, func() { l.Println("End or Crash discovered") })
									explore_end = true
									flag = false
									continue
								}
							}
							if i >= EXPLORELEN-*procs {
								//mark the processes for death
								level_print(dara.DEBUG, func() { l.Printf("Ending Execution\n") })
								procchan[ProcID].Run = -4 //mark process for death
							} else {
								procchan[ProcID].Run = -1 //lock process
							}
							procs := getDaraProcs()
							nextProc := explore_unit.GetNextThread(procs, events)
							ProcID = nextProc.ProcID
							level_print(dara.DEBUG, func() { l.Println("Chosen Proc ", ProcID, " Thread : ", nextProc.Thread.Gid) })
							procchan[ProcID].Run = -5
							procchan[ProcID].RunningRoutine = nextProc.Thread
							flag = false
						}
						//l.Print("Still running")
						atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED)
					}
					time.Sleep(time.Microsecond)
				}
			}
		}
	}
	level_print(dara.INFO, func() { l.Println("Exploration ended") })
	err := explore_unit.SaveVisitedSchedules()
	if err != nil {
		level_print(dara.FATAL, func() { l.Fatal(err) })
	}
}

// init_global_scheduler initializes the global scheduler
// This includes setting up the shared memory, initialising the global variables
func init_global_scheduler() {
	//Set up logger
	l = log.New(os.Stdout, "[Scheduler]", log.Lshortfile)
	checkargs()
	var err2 error
	prop_file := os.Getenv("PROP_FILE")
	if prop_file != "" {
		level_print(dara.INFO, func() { l.Println("Using property file", prop_file) })
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
    level_print(dara.INFO, func() { l.Println("Size of DaraProc struct is", unsafe.Sizeof(procchan[0]))})
	for i := 0; i <= *procs; i++ {
        level_print(dara.DEBUG, func() {l.Println("Initializing", i) })
		procchan[i].Lock = dara.LOCKED
		procchan[i].SyscallLock = dara.UNLOCKED
		procchan[i].Run = -1
		procchan[i].Syscall = -1
        procchan[i].CoverageIndex = 0
	}
	procStatus = make(map[int]ProcStatus)
	lockStatus = make(map[int]bool)
	linuxpids = make(map[int]uint64)
	for i := 1; i <= *procs; i++ {
		procStatus[i] = ALIVE
		lockStatus[i] = true // By default, global scheduler owns the locks on every local scheduler
		linuxpids[i] = uint64(0)
	}
	level_print(dara.INFO, func() { l.Println("Starting the Scheduler") })

}

func main() {
	init_global_scheduler()
	if *replay {
		level_print(dara.INFO, func() { l.Println("Started replaying") })
		replay_sched()
		level_print(dara.INFO, func() { l.Println("Finished replaying") })
	} else if *record {
		level_print(dara.INFO, func() { l.Println("Started recording") })
		record_sched()
		level_print(dara.INFO, func() { l.Println("Finished recording") })
	} else if *explore {
		explore_sched()
		level_print(dara.INFO, func() { l.Println("Finished exploring") })
	}
	level_print(dara.INFO, func() { l.Println("Exiting scheduler") })
}
