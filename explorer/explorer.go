package explorer

import (
	"dara"
	"encoding/gob"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"
	"log"
	"sort"
	"strings"
)

// Strategy represents the exploration strategy currently deployed
type Strategy int

// Possible enum values for Strategy
const (
	// RANDOM strategy does random exploration
	RANDOM Strategy = iota
	COVERAGE_UNIQUE
	COVERAGE_FREQUENCY
	COVERAGE_NODE_FREQUENCY
)

var (
	LogLevel int
)

// ActionType represents the various actions that can be taken by the explorer
type ActionType int

// Possible Actions
const (
	RESTART ActionType = iota
	SCHEDULE
	TIMER
)

// Action represents an action that can be scheduled by the system
type Action struct {
	Type ActionType
	Arg interface{}
}

func (a *Action) String() string {
	var retString string
	switch a.Type {
	case RESTART:
		return "RESTART"
	case SCHEDULE:
		if ptr, ok := a.Arg.(*ProcThread); ok {
			return "SCHEDULE " + ptr.String()
		}
		return "SCHEDULE"
	case TIMER:
		if ptr, ok := a.Arg.(*ProcTimer); ok {
			return "TIMER " + ptr.String()
		}
		return "TIMER"
	}
	// Should never reach here
	return retString
}

// ProcThread represents a unique goroutine in the system
// The uniqueness is given by a combination of the ProcessID and Goroutine ID
type ProcThread struct {
	ProcID int
	Thread dara.RoutineInfo
}

// ProcTimer represents a unique timer in the system
// The unique ness is given by a combination of the ProcessID and Timer ID
type ProcTimer struct {
	ProcID int
	ID int64
}

// String returns a unique representation for a goroutine in the distributed system
// The unique string is "P" + DaraProcessID + "Ti" + timerID
func (p *ProcTimer) String() string {
	return "P" + strconv.Itoa(p.ProcID) + "Ti" + strconv.FormatInt(p.ID, 10)
}

// String returns a unique string representation for a goroutine in the distributed system
// The unique string is "P" + DaraProcessID + "T" + goroutineID + "H" + hex_representation of Gpc
func (p *ProcThread) String() string {
	hex := fmt.Sprintf("%x", p.Thread.Gpc)
	return "P" + strconv.Itoa(p.ProcID) + "T" + strconv.Itoa(p.Thread.Gid) + "H" + hex
}

// GetStatus returns the status of the specific goroutine
func (p *ProcThread) GetStatus() dara.DaraProcStatus {
	return dara.GetDaraProcStatus(p.Thread.Status)
}

func dprint(level int, pfunc func()) {
	if LogLevel == dara.OFF {
		return
	}

	if level >= LogLevel {
		pfunc()
	}
}

// State represents a unique state in the system
type State string

// Explorer is the exploration unit that performs the exploration
type Explorer struct {
	logFilePath string
	visitedScheds map[string]bool
	maxDepth int
	strategy Strategy
	uniqueStates int64
	currentDepth int
	currentSchedule string
	currentProcChoice int
	seed *dara.Schedule
	currentCoverage map[int]*CovStats
	totalCoverage map[int]*CovStats
	actionStats map[string]*CovStats
	currentAction string
	initStatus  map[int]bool
	stateSpaceResults []int64
	runningProc int
	exploredStates map[State]bool
}

// loadVisitedSchedule loads all the previously seen schedules
// This is a way of persisting exploration information across multiple
// exploration executions
func (e *Explorer) loadVisitedSchedules() (err error) {
	visited := make(map[string]bool)
	_, err = os.Stat(e.logFilePath)
	if err != nil {
		// There is no log file containing previous runs
		e.visitedScheds = visited
		return nil
	}

	f, err := os.Open(e.logFilePath)
	if err != nil {
		return err
	}

	defer f.Close()

	decoder := gob.NewDecoder(f)
	if err = decoder.Decode(&visited); err != nil {
		return err
	}

	e.visitedScheds = visited
	return nil
}

// init initializes the explorer
func (e *Explorer) init() error {
	e.currentDepth = 0
	e.currentSchedule = ""
	// TODO: Make this an option
	/*
	err := e.loadVisitedSchedules()
	if err != nil {
		return err
	}
	e.uniqueStates = int64(len(e.visitedScheds))
	*/
	e.uniqueStates = 0
	return nil
}

// RestartExploration restarts the exploration from scratch
// Explorer essentially resets its state.
func (e *Explorer) RestartExploration() {
	e.visitedScheds[e.currentSchedule] = true
	e.currentDepth = 0
	e.currentSchedule = ""
	// Reset the coverage for the current run
	e.currentCoverage = make(map[int]*CovStats)
	for i := 1; i <= len(e.totalCoverage); i++ {
		covStats := make(CovStats)
		e.currentCoverage[i] = &covStats
		e.initStatus[i] = false
	}
	
	dprint(dara.INFO, func() {log.Println("Num Unique States:", e.uniqueStates)})
	e.stateSpaceResults = append(e.stateSpaceResults, e.uniqueStates)
}

// SaveVisitedSchedules saves the already seen schedules in a log file
// This is to persist state of the explorer across multiple invocations
// Currently not enabled.
// TODO: Make saving/using of old run files an option.
func (e *Explorer) SaveVisitedSchedules() error {
	_, err := os.Stat(e.logFilePath)
	if err != nil {
		f, err := os.Create(e.logFilePath)
		if err != nil {
			return err
		}
		f.Close()
	}

	f, err := os.OpenFile(e.logFilePath, os.O_WRONLY, 0777)
	if err != nil {
		return err
	}

	defer f.Close()

	encoder := gob.NewEncoder(f)
	if err := encoder.Encode(e.visitedScheds); err != nil {
		return err
	}

	return nil
}

// SetMaxDepth sets the maximum depth for Depth-Based Exploration
func (e *Explorer) SetMaxDepth(depth int) {
	e.maxDepth = depth
}

func (e *Explorer) PrintCurrentCoverage() {
	for _, v := range e.currentCoverage {
		log.Println(v)
	}
}

func (e *Explorer) SaveStateSpaceResults(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	file.WriteString("States\n")
	for _, val := range e.stateSpaceResults {
		file.WriteString(strconv.FormatInt(val, 10) + "\n")
	}
	return nil
}

func (e *Explorer) getNextCoverageThreadHelper(threads *[]ProcThread, obj Objective) *ProcThread {
	var procThread *ProcThread

	minTotalCoverageScore := CalcFullCoverageScore(e.totalCoverage, obj)
	//log.Println("TotalCoverageScore", minTotalCoverageScore)
	prevScore := minTotalCoverageScore
	var noActionThreads []ProcThread
	for _, thread := range *threads {
		procString := thread.String()
		// Check if we have any information about this action
		if v, ok := e.actionStats[procString]; ok {
			// Check how the coverage score will change with this 
			estimate := GetCoverageEstimate(e.totalCoverage, v, thread.ProcID, obj)
			//log.Println("Estimate is", estimate)
			if estimate > minTotalCoverageScore {
				minTotalCoverageScore = estimate
				procThread = &thread
			}
		} else {
			// We have no information about this thread.....
			noActionThreads = append(noActionThreads, thread)
		}
	}
	if minTotalCoverageScore != prevScore {
		dprint(dara.INFO, func(){log.Println("[Explorer]Chose total coverage action")})
		return procThread
	}
	// Choose a thread if we have no information about it before
	if len(noActionThreads) > 0 {
		// Choose a thread at random from the threads we have not seen before
		dprint(dara.INFO, func(){log.Println("[Explorer]Choose unseen action for info")})
		return getNextRandomThread(&noActionThreads)
	} 
	// Pick an action that might increase local coverage
	// Also try not choosing the same action again from a previous path!
	minCoverageScore := CalcFullCoverageScore(e.currentCoverage, obj)
	prevScore = minCoverageScore
	actionFound := false
	for _, thread := range *threads {
		procString := thread.String()
		// Check if scheduling this action would take us in a prerviously
		// seen state
		currentSchedule := e.currentSchedule + procString
		if _, ok := e.visitedScheds[currentSchedule]; ok {
			// Have already seen this state... Try exploring something else
			continue
		}
		// Check if we have any information about this action
		// At this point we must have!
		if v, ok := e.actionStats[procString]; ok {
			// Check how the coverage score will change with this 
			estimate := GetCoverageEstimate(e.currentCoverage, v, thread.ProcID, obj)
			if estimate > minCoverageScore {
				actionFound = true
				minCoverageScore = estimate
				procThread = &thread
			}
		}
	}
	if minCoverageScore != prevScore || actionFound {
		dprint(dara.INFO, func(){log.Println("[Explorer]Chose local coverage aciton")})
		return procThread
	}
	
	// At this point no action is going to increase total coverage,
	// no new local coverage path, and no threads with no prior information
	// Default to picking a thread at random!
	return getNextRandomThread(threads)
}

func (e *Explorer) getNextCoverageThread(threads *[]ProcThread) *ProcThread {
	return e.getNextCoverageThreadHelper(threads, UNIQUE)
}

func (e *Explorer) getNextCoverageFreqThread(threads *[]ProcThread) *ProcThread {
	return e.getNextCoverageThreadHelper(threads, FREQUENCY)
}

func (e *Explorer) getNextCoverageFreqRoleThread(threads *[]ProcThread) *ProcThread {
	return e.getNextCoverageThreadHelper(threads, NODE_FREQUENCY)
}

// getNextRandomThread returns the next goroutine to be schedule
// The next goroutine is selected as a random choice
func getNextRandomThread(threads *[]ProcThread) *ProcThread {
	choice := rand.Intn(len(*threads))
	dprint(dara.DEBUG, func(){log.Println("Chosen thread is", (*threads)[choice].Thread.Gid)})
	dprint(dara.INFO, func(){log.Println("[Explorer]Chose random action")})
	return &(*threads)[choice]
}

// getNextRandomProcess returns the next process on which a goroutine will be scheduled
func (e *Explorer) getNextRandomProcess(numProcs int) int {
	return rand.Intn(numProcs) + 1
}

// getNextPossibleThreads returns all the possible schedules that
// can be scheduled next. A goroutine is schedule iff the state of the goroutine
// is dara.Runnable
func (e *Explorer) getNextPossibleThreads(threads *[]ProcThread, clocks *map[int]*VirtualClock, isblocked map[int]bool) *[]ProcThread {
	possibleThreads := []ProcThread{}

	for i := 0; i < len(*threads); i++ {
		dprint(dara.DEBUG, func() {log.Println("[Explorer]Thread", (*threads)[i].Thread.Gid, "has status", dara.GStatusStrings[(*threads)[i].GetStatus()])})
		if v, ok := e.initStatus[(*threads)[i].ProcID]; ok && v {
			if (*threads)[i].GetStatus() == dara.Runnable {
				// check that the goroutine is not on a sleeping queue
				if (*clocks)[(*threads)[i].ProcID].IsSleeping((*threads)[i].Thread.Gid) {
					continue
				}
				if isblocked[(*threads)[i].ProcID] {
					continue
				}
				possibleThreads = append(possibleThreads, (*threads)[i])
			}
		} else {
			// If the process hasn't been initialized then we can only select the first goroutine for execution!
			if (*threads)[i].Thread.Gid == 1 {
				possibleThreads = append(possibleThreads, (*threads)[i])
			}
		}
	}
    dprint(dara.DEBUG, func() { log.Println("Length of possible threads", len(possibleThreads))})
	return &possibleThreads
}

// GetNextThread returns the next goroutine to be scheduled based on the previously selected strategy
func (e *Explorer) GetNextThread(nextPossibleThreads *[]ProcThread, events *[]dara.Event, coverage *CovStats) *ProcThread {

	// Switch over different strategies. 
	var nextThread *ProcThread
	switch e.strategy {
		case RANDOM:
			nextThread = getNextRandomThread(nextPossibleThreads)
		case COVERAGE_UNIQUE:
			nextThread = e.getNextCoverageThread(nextPossibleThreads)
		case COVERAGE_FREQUENCY:
			nextThread = e.getNextCoverageFreqThread(nextPossibleThreads)
		case COVERAGE_NODE_FREQUENCY:
			nextThread = e.getNextCoverageFreqRoleThread(nextPossibleThreads)
	}
	e.currentSchedule = e.currentSchedule + nextThread.String()
	if _, ok := e.visitedScheds[e.currentSchedule]; !ok {
		e.visitedScheds[e.currentSchedule] = true
		e.currentDepth += 1
	}
	return nextThread
}

// noWaitingEvents returns true if there are no events in the waitingQ for any of the virtual clocks
func (e *Explorer) noWaitingEvents(clocks *map[int]*VirtualClock) bool {
	for _, vc := range *clocks {
		if len(vc.WaitingQ) != 0 {
			// We can return false as there is still something left to be done
			return false
		}
	}
	return true
}

func (e *Explorer) GetNextTimer(clock *VirtualClock, ProcID int) *ProcTimer {
	var procTimer *ProcTimer
	event := clock.GetNextEvent()
	if event == nil {
		return nil
	}
	procTimer = &ProcTimer{ProcID: ProcID, ID: event.TimerID}
	e.currentSchedule = e.currentSchedule + procTimer.String()
	if _, ok := e.visitedScheds[e.currentSchedule]; !ok {
		e.visitedScheds[e.currentSchedule] = true
		e.currentDepth += 1
	}
	return procTimer
}

// hasWaitingEvents checks if there are any waiting events left for a process' virtual clock
func (e *Explorer) hasWaitingEvents(clock *VirtualClock) bool {
	return len(clock.WaitingQ) != 0
}

// BuildActionStats builds the action history from the seed schedule.
// This is important as it prevents 
func (e *Explorer) BuildActionStats() {
	for _, event := range e.seed.CovEvents {
		logEvent := e.seed.LogEvents[event.EventIndex]
		hex := fmt.Sprintf("%x", logEvent.G.Gpc)
		actionString := "P" + strconv.Itoa(logEvent.P) + "T"+ strconv.Itoa(logEvent.G.Gid) + "H" + hex
		//log.Println(actionString)
		coverage := make(CovStats)
		for k, v := range event.CoverageInfo {
			coverage[k] = v
		}
		if v, ok := e.actionStats[actionString]; !ok {
			e.actionStats[actionString] = &coverage
		} else {
			CoverageUnion(v, &coverage)
		}
	}
	//log.Println("Finished building")
}

// UpdateCurrentActionStats merge the statistics for coverage for all the actions
func (e *Explorer) UpdateCurrentActionStats(events *[]dara.Event, coverage *CovStats) {
	if v, ok := e.actionStats[e.currentAction]; !ok {
		e.actionStats[e.currentAction] = coverage
	} else {
		// Merge the coverage
		CoverageUnion(v, coverage)
	}
}

func (e *Explorer) UpdateActionCoverage(events *[]dara.Event, coverage *CovStats) {
	if e.currentAction != "" || e.currentAction != "RESTART" {
		if coverage != nil {
			e.UpdateCurrentActionStats(events, coverage)
		}
		if e.runningProc != 0 {
			// Update the coverage of the current run
			CoverageUnion(e.currentCoverage[e.runningProc], coverage)
			// Update the total coverage
			CoverageUnion(e.totalCoverage[e.runningProc], coverage)
		}
		// Update the unique states counter if we have not seen this state before
		var currentState State
		var procStates []string
		for i := 1; i <= len(e.currentCoverage); i++ {
			var procState string
			procState = "P" + strconv.Itoa(i)
			var keys []string
			cov := e.currentCoverage[i]
			for k := range *cov {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var coverageStrings []string
			for _, key := range keys {
				coverageStrings = append(coverageStrings, key + ":" + strconv.FormatUint((*cov)[key], 10))
			}
			procState = procState + "," + strings.Join(coverageStrings, ",")
			procStates = append(procStates, procState)
		}
		currentState = State(strings.Join(procStates, ","))
		if _, ok := e.exploredStates[currentState]; !ok {
			e.uniqueStates += 1
			e.exploredStates[currentState] = true
		} 
	}
}

// GetNextAction gets the next action to be executed
// threads: List of all goroutines in the system
// events: Events that were executed by the last action
// coverage: Coverage statistics of the last action
// clocks: Virtual Clocks of all the processes
// allCoverage: Total Coverage across all nodes across all runs
func (e *Explorer) GetNextAction(threads *[]ProcThread, events *[]dara.Event, coverage *CovStats, clocks *map[int]*VirtualClock, isblocked map[int]bool) Action {
	// Choose next action
	nextPossibleThreads := e.getNextPossibleThreads(threads, clocks, isblocked)
	if e.currentDepth >= e.maxDepth || (len(*nextPossibleThreads) == 0 && e.noWaitingEvents(clocks)) {
		// Return action to restart exploration!
		e.RestartExploration()
		e.currentAction = "RESTART"
		e.runningProc = 0
		return Action{Type: RESTART}
	}

	// Choose between scheduling a thread and firing off a timer
	choice := rand.Intn(2)
	// See if a timer that can be fired on any process.
	// If not then move to scheduling a thread.
	// If there are no possible threads then we might need to wake something up.
	if choice == 0 || len(*nextPossibleThreads) == 0 {
		for id, clock := range *clocks {
			if e.hasWaitingEvents(clock){
				nextTimer := e.GetNextTimer(clock, id)
				if nextTimer == nil {
					// We possibly woke up a goroutine but that is about it.
					nextPossibleThreads = e.getNextPossibleThreads(threads, clocks, isblocked)
					continue
				}
				e.currentAction = nextTimer.String()
				e.runningProc = nextTimer.ProcID
				return Action{Type: TIMER, Arg: nextTimer}
			}
		}
	}

	nextThread := e.GetNextThread(nextPossibleThreads, events, coverage)
	if e.initStatus[nextThread.ProcID] == false {
		e.initStatus[nextThread.ProcID] = true
	}
	e.currentAction = nextThread.String()
	e.runningProc = nextThread.ProcID
	return Action{Type: SCHEDULE, Arg: nextThread}
}

// GetNextProcess returns the next process to be scheduled based on the selected strategy
func (e *Explorer) GetNextProcess(numProcs int) int {
	nextProc := e.getNextRandomProcess(numProcs)
	e.currentProcChoice = nextProc
	return nextProc
}

// MountExplorer initializes an explorer
func MountExplorer(logFilePath string, explorationStrategy Strategy, seed * dara.Schedule, numProcs int, level int) (*Explorer, error) {
	// Set the seed to something random
	rand.Seed(time.Now().UnixNano())
	// Initialize the explorer
	e := &Explorer{logFilePath : logFilePath, strategy : explorationStrategy, seed: seed, currentCoverage: make(map[int]*CovStats), actionStats: make(map[string]*CovStats), totalCoverage: make(map[int]*CovStats), initStatus: make(map[int]bool)}
	for i := 1; i <= numProcs; i++ {
		covStats := make(CovStats)
		covStatsTotal := make(CovStats)
		e.currentCoverage[i] = &covStats
		e.totalCoverage[i] = &covStatsTotal
		e.initStatus[i] = false // Process hasn't been initialized yet
	}
	e.exploredStates = make(map[State]bool)
	e.BuildActionStats()
	e.visitedScheds = make(map[string]bool)
	LogLevel = level
	err := e.init()
	if err != nil {
		return nil, err
	}
	return e, nil
}
