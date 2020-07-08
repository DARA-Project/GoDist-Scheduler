package explorer

import (
	"dara"
	"encoding/gob"
	"fmt"
	"math/rand"
	"os"
	"strconv"
    "log"
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

// ProcThread represents a unique goroutine in the system
// The uniqueness is given by a combination of the ProcessID and Goroutine ID
type ProcThread struct {
	ProcID int
	Thread dara.RoutineInfo
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

// Explorer is the exploration unit that performs the exploration
type Explorer struct {
	logFilePath string
	visitedScheds map[string]bool
	maxDepth int
	strategy Strategy
	uniqueStates int64
	currentDepth int
	currentSchedule string
	seed *dara.Schedule
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
	e.maxDepth = 0
	e.currentDepth = 0
	e.currentSchedule = ""
	err := e.loadVisitedSchedules()
	if err != nil {
		return err
	}
	e.uniqueStates = int64(len(e.visitedScheds))
	return err
}

// RestartExploration restarts the exploration from scratch
// Explorer essentially resets its state.
func (e *Explorer) RestartExploration() {
	e.visitedScheds[e.currentSchedule] = true
	e.maxDepth = 0
	e.currentDepth = 0
	e.currentSchedule = ""
}

// SaveVisitedSchedules saves the already seen schedules in a log file
// This is to persist state of the explorer across multiple invocations
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

// getNextRandomThread returns the next goroutine to be schedule
// The next goroutine is selected as a random choice
func getNextRandomThread(threads *[]ProcThread) *ProcThread {
    if len(*threads) == 0 {
        log.Fatal("No possible threads to be run. Possible deadlock detected")
    }
	choice := rand.Intn(len(*threads))
    log.Println("Chosen thread is", (*threads)[choice].Thread.Gid)
	return &(*threads)[choice]
}

// getNextRandomProcess returns the next process on which a goroutine will be scheduled
func (e *Explorer) getNextRandomProcess(numProcs int) int {
	return rand.Intn(numProcs) + 1
}

// getNextPossibleThreads returns all the possible schedules that
// can be scheduled next. A goroutine is schedule iff the state of the goroutine
// is dara.Runnable
func (e *Explorer) getNextPossibleThreads(threads *[]ProcThread) *[]ProcThread {
	possibleThreads := []ProcThread{}

	for i := 0; i < len(*threads); i++ {
        log.Println("Thread", (*threads)[i].Thread.Gid, "has status", dara.GStatusStrings[(*threads)[i].GetStatus()])
		if (*threads)[i].GetStatus() == dara.Runnable {
			possibleThreads = append(possibleThreads, (*threads)[i])
		}
	}
    log.Println("Length of possible threads", len(possibleThreads))
	return &possibleThreads
}

// GetNextThread returns the next goroutine to be scheduled based on the previously selected strategy
func (e *Explorer) GetNextThread(threads *[]ProcThread, events *[]dara.Event, coverage *CovStats) *ProcThread {
	nextPossibleThreads := e.getNextPossibleThreads(threads)

	// TODO: Switch over different strategies. Currently only random is implemented
	nextThread := getNextRandomThread(nextPossibleThreads)
	e.currentSchedule += nextThread.String()
	if _, ok := e.visitedScheds[e.currentSchedule]; !ok {
		e.visitedScheds[e.currentSchedule] = true
		e.uniqueStates += 1
	}

	return nextThread
}

// GetNextProcess returns the next process to be scheduled based on the selected strategy
func (e *Explorer) GetNextProcess(numProcs int) int {
	// TODO: Switch over different strategies. Currently only random is implemented
	nextProc := e.getNextRandomProcess(numProcs)
	return nextProc
}

// MountExplorer initializes an explorer
func MountExplorer(logFilePath string, explorationStrategy Strategy, seed * dara.Schedule) (*Explorer, error) {
	e := &Explorer{logFilePath : logFilePath, strategy : explorationStrategy, seed: seed}
	err := e.init()
	if err != nil {
		return nil, err
	}
	return e, nil
}
