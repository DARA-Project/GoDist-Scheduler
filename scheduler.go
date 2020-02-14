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
	"os"
	"runtime"
	"sync/atomic"
    "strconv"
	"time"
	"unsafe"
)

//Command line arguments
var (
	procs = flag.Int("procs", 1, "The number of processes to model check")
	record = flag.Bool("record", false, "Record an execution")
	replay = flag.Bool("replay", false, "Replay an execution")
	explore = flag.Bool("explore", false, "Explore a recorded execution")
    sched_file = flag.String("schedule", "", "The schedule file where the schedule will be stored/loaded from")
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
    LogLevel int
    //Variable to hold propertychecker
    checker *propchecker.Checker
)

var (
	l *log.Logger
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
    level := os.Getenv("DARA_LOG_LEVEL")
    i, err := strconv.Atoi(level)
    if err != nil {
        l.Fatal("Invalid log level specified")
    }
    LogLevel = int(i)
	return
}

func forward() {
    time.Sleep(1 *time.Millisecond)
}

func level_print(level int, pfunc func()) {
    if LogLevel == dara.OFF {
        return
    }

    if level >= LogLevel {
        pfunc()
    }
}

var schedule dara.Schedule

func roundRobin() int {
	LastProc = (LastProc + 1) % (*procs + 1)
    // Dara doesn't have ProcID 0
	if LastProc == 0 {
		LastProc++
	}
	return LastProc
}

func getSchedulingEvents(sched dara.Schedule) map[int][]dara.Event {
    proc_event_map := make(map[int][]dara.Event)
    for i := 0; i < len(sched); i++ {
        eventTypeString := common.EventTypeString(sched[i].Type)
        if eventTypeString == "SCHEDULE" {
            // Don't want to run timer events
            if string(bytes.Trim(sched[i].G.FuncInfo[:64],"\x00")[:]) != "runtime.timerproc" {
                proc_event_map[sched[i].P] = append(proc_event_map[sched[i].P], sched[i])
            }
        }
    }
    return proc_event_map
}

func ConsumeAndPrint(ProcID int, context * map[string]interface{}) []dara.Event{
	cl := ConsumeLog(ProcID, context)

	return cl
}

func ConsumeLog(ProcID int, context * map[string]interface{}) []dara.Event {
    // The 2 print statements somehow have different values for SimpleFileRead. How the fuck.....
    level_print(dara.INFO, func() { l.Println("Current log event index is", procchan[ProcID].LogIndex) })
	logsize := procchan[ProcID].LogIndex
    level_print(dara.INFO, func() { l.Println("Current log event index is", logsize) })
	log := make([]dara.Event,logsize)
	for i:=0;i<logsize;i++ {
		ee := &(procchan[ProcID].Log[i])
		e := &(log[i])
		(*e).Type = (*ee).Type
		(*e).P = (*ee).P
		(*e).G = (*ee).G
		(*e).Epoch = (*ee).Epoch
		(*e).LE.LogID = string(bytes.Trim((*ee).ELE.LogID[:dara.VARBUFLEN], "\x00")[:])
		(*e).LE.Vars = make([]dara.NameValuePair,(*ee).ELE.Length)
		for j:=0;j<len((*e).LE.Vars);j++{
			(*e).LE.Vars[j].VarName = string(bytes.Trim((*ee).ELE.Vars[j].VarName[:dara.VARBUFLEN], "\x00")[:])
			(*e).LE.Vars[j].Value = runtime.DecodeValue((*ee).ELE.Vars[j].Type,(*ee).ELE.Vars[j].Value)
			(*e).LE.Vars[j].Type = string(bytes.Trim((*ee).ELE.Vars[j].Type[:dara.VARBUFLEN], "\x00")[:])
            // Build Up context for property checking
            (*context)[e.LE.Vars[j].VarName] = e.LE.Vars[j].Value
		}
		(*e).SyscallInfo = (*ee).SyscallInfo
		//(*e).Msg = (*ee).EM //TODO complete messages
        level_print(dara.DEBUG, func() {l.Println(common.ConciseEventString(e))})
        if e.Type == dara.THREAD_EVENT {
            level_print(dara.DEBUG, func() { l.Println("Thread creation noted with ID", e.G.Gid, "at function", string(e.G.FuncInfo[:64]))})
        }
	}
	procchan[ProcID].LogIndex = 0
	procchan[ProcID].Epoch++
	return log
}

func CheckAllGoRoutinesDead(ProcID int) bool {
    allDead := true
    level_print(dara.DEBUG, func() { l.Printf("Checking if all goroutines are dead yet for Process %d\n", ProcID)})
    //l.Printf("Num routines are %d", len(procchan[ProcID].Routines))
    for _, routine := range procchan[ProcID].Routines {
        // Only check for dead goroutines for real go routines
        if routine.Gid != 0 {
            status := dara.GetDaraProcStatus(routine.Status)
            level_print(dara.DEBUG, func() {l.Printf("Status of GoRoutine %d is %s\n", routine.Gid, dara.GStatusStrings[status])})
            allDead = allDead && (status == dara.Dead)
        }
    }
    return allDead
}

func GetAllRunnableRoutines(event dara.Event) []int {
	runnable := make([]int,0)
	for j, info := range procchan[event.P].Routines {
		if dara.GetDaraProcStatus(info.Status) != dara.Idle {
			level_print(dara.DEBUG, func () {l.Printf("Proc[%d]Routine[%d].Info = %s",event.P,j,common.RoutineInfoString(&info))})
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

func CompareEvents(e1 dara.Event, e2 dara.Event) bool {
    // TODO Make a more detailed comparison
    if e1.Type != e2.Type {
        return false
    }
    return true
}

func CheckEndOrCrash(e dara.Event) bool {
    if e.Type == dara.END_EVENT || e.Type == dara.CRASH_EVENT {
        return true
    }
    return false
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

func check_properties(context map[string]interface{}) (bool, error) {
    if checker != nil {
        return checker.Check(context)
    }
    return true, nil
}

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
	level_print(dara.DEBUG, func () {l.Println("Num Events : ", len(schedule))})
    level_print(dara.DEBUG, func () {l.Println("Replaying the following schedule : ", common.ConciseScheduleString(&schedule))})
    fast_replay := os.Getenv("FAST_REPLAY")
    if fast_replay == "true" {
        proc_schedule_map := getSchedulingEvents(schedule)
        preload_replay_schedule(proc_schedule_map)
        for procID := range procchan {
            if procchan[procID].Run == -1 && procchan[procID].LogIndex == 0 {
                procchan[procID].Run = -100
            } else {
                level_print(dara.INFO, func() {l.Println("Enabling runtime for proc", procID)})
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
        level_print(dara.INFO, func() {l.Println("Fast Replay done")})
    } else {
	    for i<len(schedule) {
	    	if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].P].Lock))),dara.UNLOCKED,dara.LOCKED) {
	    		if procchan[schedule[i].P].Run == -1 { //check predicates on goroutines + schedule

	    			//move forward
	    			forward()

	    			level_print(dara.DEBUG, func () {l.Print("Scheduling Event")})
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
                        check_replayable:
                        can_run := Is_event_replayable(runnable, schedule[i].G.Gid)
                        if !can_run {
                            if string(bytes.Trim(schedule[i].G.FuncInfo[:64],"\x00")[:]) != "runtime.timerproc" {
                                level_print(dara.INFO, func() {l.Println("G info was ", string(schedule[i].G.FuncInfo[:64]))})
                                level_print(dara.FATAL, func () {l.Fatal("Can't run event ", i, ": ",common.ConciseEventString(&schedule[i]))})
                            } else {
                                //Trying to schedule timer thread. Let's just increment the event counter
                                level_print(dara.INFO, func() {l.Println("G info was ", string(schedule[i].G.FuncInfo[:64]))})
                                i++
                                //Check if the new event is replayable
                                goto check_replayable
                            }
                        }
	    				runningindex = schedule[i].G.Gid
	    			}
                    _ = runningindex
	    			//Assign which goroutine to run
	    			procchan[schedule[i].P].Run = -5
	    			procchan[schedule[i].P].RunningRoutine = schedule[i].G
                    level_print(dara.INFO, func () { l.Println("Replaying Event :", common.ConciseEventString(&schedule[i]))})
	    			level_print(dara.INFO, func () { l.Printf("Running (%d/%d) %d %s",i+1,len(schedule),schedule[i].P,common.RoutineInfoString(&schedule[i].G))})

	    			atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].P].Lock))),dara.UNLOCKED)
	    			for {
	    				if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].P].Lock))),dara.UNLOCKED,dara.LOCKED) {
                            currentDaraProc := schedule[i].P
	    					if procchan[schedule[i].P].Run == -1 {
	    						atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[schedule[i].P].Lock))),dara.UNLOCKED)
                                events := ConsumeAndPrint(currentDaraProc, &context)
                                level_print(dara.DEBUG, func() { l.Println("Replay Consumed ", len(events), "events")})
                                for _, e := range events {
                                    same_event := CompareEvents(e, schedule[i])
                                    if !same_event {
                                        level_print(dara.WARN, func() {l.Println("Replayed 2 different events", common.ConciseEventString(&e), common.ConciseEventString(&schedule[i]))})
                                    } else {
                                        level_print(dara.DEBUG, func() {l.Println("Replayed ", common.ConciseEventString(&schedule[i]))})
                                    }
                                    i++
                                }
                                level_print(dara.DEBUG, func() {l.Println("Replay : At event", i)})
                                if i >= len(schedule) {
                                    level_print(dara.DEBUG, func() { l.Printf("Informing the local scheduler end of replay")})
                                    procchan[currentDaraProc].Run = -4
                                    break
                                }
	    						break
	    					} else if procchan[schedule[i].P].Run == -100 {
                                // This means that the local runtime finished and that we can move on
                                events := ConsumeAndPrint(currentDaraProc, &context)
                                level_print(dara.DEBUG, func() {l.Println("Replay Consumed", len(events), "events")})
                                for _, e := range events {
                                    same_event := CompareEvents(e, schedule[i])
                                    if !same_event {
                                        level_print(dara.WARN, func() {l.Println("Replayed 2 different events", common.ConciseEventString(&e), common.ConciseEventString(&schedule[i]))})
                                    }
                                    i++
                                }
                                break
                            }
	    					atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[currentDaraProc].Lock))),dara.UNLOCKED)
	    				}
	    			}
	    			time.Sleep(time.Second)
	    		}
	    	}
	    }
    }
	level_print(dara.DEBUG, func() {l.Println("Replay is over")})
}


func record_sched() {
	LastProc = 0
	var i int
    // Context for property checking
    context := make(map[string]interface{})
	for i<RECORDLEN {
	    ProcID := roundRobin()
        level_print(dara.DEBUG, func(){l.Println("Chosen process for running is", ProcID)})
		//else busy wait
        //l.Printf("Procchan Run status is %d\n", procchan[ProcID].Run)
		if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED,dara.LOCKED) {
            level_print(dara.DEBUG, func(){l.Println("Obtained Lock with run value", procchan[ProcID].Run, "on process", ProcID)})
            if procchan[ProcID].Run == -100 {
			    events := ConsumeAndPrint(ProcID, &context)
                result, err := check_properties(context)
                if err != nil {
                    level_print(dara.INFO, func() {l.Println(err)})
                }
                if !result {
                    level_print(dara.INFO, func(){l.Println("Property check failed")})
                }
			    schedule = append(schedule,events...)
                break
            }
            if procchan[ProcID].Run == -6 {
                //Process was blocked on network, hopefully it is unblocked after a full cycle of process scheduling!
                //If not, the process will eventually grab the lock whenever it wakes up, until then scheduler will continue
                //to own the lock.
                level_print(dara.DEBUG, func() {l.Println("Releasing the lock after block back to", ProcID)})
                atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))), dara.UNLOCKED)
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
                            level_print(dara.DEBUG, func() {l.Printf("Ending discovered")})
                            events := ConsumeAndPrint(ProcID, &context)
                            result, err := check_properties(context)
                            if err != nil {
                                level_print(dara.INFO, func() {l.Println(err)})
                            }
                            if !result {
                                level_print(dara.INFO, func(){l.Println("Property check failed")})
                            }
                            schedule = append(schedule,events...)
                            flag = false
                            continue
                        }
						if procchan[ProcID].Run != -3 {
                            level_print(dara.DEBUG, func() {l.Printf("Procchan Run status inside is %d\n", procchan[ProcID].Run)})
							level_print(dara.DEBUG, func() {l.Printf("Recording Event on Process/Node %d\n",ProcID)})
							//Update the last running routine
							level_print(dara.DEBUG, func() {l.Printf("Recording Event Number %d",i)})
							events := ConsumeAndPrint(ProcID, &context)
                            result, err := check_properties(context)
                            if err != nil {
                                level_print(dara.INFO, func() {l.Println(err)})
                            }
                            if !result {
                                level_print(dara.INFO, func(){l.Println("Property check failed")})
                            }
							schedule = append(schedule,events...)
							//Set the status of the routine that just
							//ran
							if (i >= RECORDLEN - *procs) {
								//mark the processes for death
								level_print(dara.DEBUG, func() {l.Printf("Ending Execution!")})
								procchan[ProcID].Run = -4 //mark process for death
							} else if procchan[ProcID].Run != -6 {
                                //Only reset if the process is not blocked on network IO
								procchan[ProcID].Run = -1	//lock process
							}

                            level_print(dara.DEBUG, func() {l.Printf("Found %d log events", len(events))})
							i += len(events)
							flag = false
						}
						if procchan[ProcID].LogIndex > 0 {
							level_print(dara.DEBUG, func() {l.Printf("Procchan %d\n", procchan[ProcID].Run)})
							events := ConsumeAndPrint(ProcID, &context)
                            result, err := check_properties(context)
                            if err != nil {
                                level_print(dara.INFO, func() {l.Println(err)})
                            }
                            if !result {
                                level_print(dara.INFO, func(){l.Println("Property check failed")})
                            }
							schedule = append(schedule,events...)
							f, erros := os.Create(*sched_file)
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
			f, erros := os.Create(*sched_file)
			if erros != nil {
				l.Fatal(err)
			}
			enc := json.NewEncoder(f)
            level_print(dara.INFO, func() {l.Println("Schedule has : ", len(schedule), "events")})
            //for _, e := range(schedule) {
            //    l.Println(common.EventString(&e))
            //}
			enc.Encode(schedule)
            if CheckAllGoRoutinesDead(ProcID) {
                break
            }
		}
	}
	//l.Printf(common.ScheduleString(&schedule))
	//l.Printf("%+v\n",schedule)
    //level_print(dara.INFO, func() {l.Println("Recorded schedule is as follows : \n",common.ConciseScheduleString(&schedule))})
    f, err := os.Create(*sched_file)
    if err != nil {
        l.Fatal(err)
    }
    _, err = f.WriteString(common.ConciseScheduleString(&schedule))
    if err != nil {
        l.Fatal(err)
    }
	level_print(dara.DEBUG, func() {l.Println("The End")})
}

func explore_sched() {
	explore_unit := explore_init()
	level_print(dara.INFO, func() {l.Println("Dora the Explorer begins")})
	LastProc = 0
    explore_end := false
    context := make(map[string]interface{})
	var i int
	for i < EXPLORELEN {
	    ProcID := roundRobin()
        if explore_end {
            break
        }
		if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED,dara.LOCKED) {
            if procchan[ProcID].Run == -100 {
				events := ConsumeAndPrint(ProcID, &context)
				schedule = append(schedule,events...)
				i += len(events)
                // Check if one of them is a crash or end event. If so, exploration should be over.
                // As they both are ending events, we only need to check the last event in the schedule
                for _, e := range events {
                    if CheckEndOrCrash(e) {
                        level_print(dara.INFO, func() {l.Println("End or Crash discovered")})
                        explore_end = true
                        continue
                    }
                }
            }
			if procchan[ProcID].Run == -1 { //TODO check predicates on goroutines + schedule
				forward()
				procchan[ProcID].Run = -5

				atomic.StoreInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED)
				flag := true
				for ; flag; {
					if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&(procchan[ProcID].Lock))),dara.UNLOCKED,dara.LOCKED) {
						//l.Printf("procchan[schedule[%d]].Run = %d",i,procchan[schedule[i]].Run)
						if procchan[ProcID].Run == -1 {
							events := ConsumeAndPrint(ProcID, &context)
							schedule = append(schedule,events...)
							i += len(events)
                            // Check if one of them is a crash or end event. If so, exploration should be over.
                            // As they both are ending events, we only need to check the last event in the schedule
                            for _, e := range events {
                                if CheckEndOrCrash(e) {
                                    level_print(dara.INFO, func() {l.Println("End or Crash discovered")})
                                    explore_end = true
                                    flag = false
                                    continue
                                }
                            }
							if (i >= EXPLORELEN - *procs) {
								//mark the processes for death
								level_print(dara.DEBUG, func() {l.Printf("Ending Execution\n")})
								procchan[ProcID].Run = -4 //mark process for death
							} else {
								procchan[ProcID].Run = -1	//lock process
							}
							procs := getDaraProcs()
							nextProc := explore_unit.GetNextThread(procs,events)
							ProcID = nextProc.ProcID
                            level_print(dara.DEBUG, func() {l.Println("Chosen Proc ", ProcID, " Thread : ",nextProc.Thread.Gid)})
							procchan[ProcID].Run = -5
							procchan[ProcID].RunningRoutine = nextProc.Thread
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
    level_print(dara.INFO, func() {l.Println("Exploration ended")})
    err := explore_unit.SaveVisitedSchedules()
    if err != nil {
        level_print(dara.FATAL, func() {l.Fatal(err)})
    }
}

func init_global_scheduler() {
	//Set up logger
	l = log.New(os.Stdout,"[Scheduler]",log.Lshortfile)
	checkargs()
    var err2 error
    prop_file := os.Getenv("PROP_FILE")
    if prop_file != "" {
	    level_print(dara.INFO, func(){l.Println("Using property file", prop_file)})
        checker, err2 = propchecker.NewChecker(prop_file)
        if err2 != nil {
            l.Fatal(err2)
        }
    }
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
	level_print(dara.INFO, func(){l.Println("Starting the Scheduler")})

}

func main() {
    init_global_scheduler()
	if *replay {
		level_print(dara.INFO, func(){l.Println("Started replaying")})
		replay_sched()
		level_print(dara.INFO, func(){l.Println("Finished replaying")})
	} else if *record {
		level_print(dara.INFO, func(){l.Println("Started recording")})
		record_sched()
		level_print(dara.INFO, func(){l.Println("Finished recording")})
	} else if *explore {
		explore_sched()
		level_print(dara.INFO, func(){l.Println("Finished exploring")})
	}
	level_print(dara.INFO, func(){l.Println("Exiting scheduler")})
}


//Printing Functions
