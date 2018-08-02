package explorer

import (
	"dara"
	"encoding/gob"
	"fmt"
	"math/rand"
	"os"
	"strconv"
)

type Strategy int

const (
	RANDOM Strategy = iota
	BFS
	DFS
	// DPOR
	// DPOR + RANDOM
)

type ProcThread struct {
	ProcID int
	Thread dara.RoutineInfo
}

func (p *ProcThread) String() string {
	hex := fmt.Sprintf("%x", p.Thread.Gpc)
	return "P" + strconv.Itoa(p.ProcID) + "T" + strconv.Itoa(p.Thread.RoutineCount) + "H" + hex
}

func (p *ProcThread) GetStatus() dara.DaraProcStatus {
	return dara.GetDaraProcStatus(p.Thread.Status)
}

type Explorer struct {
	logFilePath string
	visitedScheds map[string]bool
	maxDepth int
	strategy Strategy
	uniqueStates int64
	currentDepth int
	currentSchedule string
}

func (e *Explorer) loadVisitedSchedules() (visited map[string]bool, err error) {
	_, err = os.Stat(e.logFilePath)
	if err != nil {
		return map[string]bool{}, nil
	}

	f, err := os.Open(e.logFilePath)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	decoder := gob.NewDecoder(f)
	if err = decoder.Decode(&visited); err != nil {
		return nil, err
	}

	return visited, nil
}

func (e *Explorer) init() error {
	e.maxDepth = 0
	e.currentDepth = 0
	e.currentSchedule = ""
	var err error
	e.visitedScheds, err = e.loadVisitedSchedules()
	e.uniqueStates = int64(len(e.visitedScheds))
	return err
}

func (e *Explorer) RestartExploration() {
	e.visitedScheds[e.currentSchedule] = true
	e.maxDepth = 0
	e.currentDepth = 0
	e.currentSchedule = ""
}

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

func (e *Explorer) SetMaxDepth(depth int) {
	e.maxDepth = depth
}

func getNextRandomThread(threads []ProcThread) ProcThread {
	choice := rand.Intn(len(threads))
	return threads[choice]
}

func (e *Explorer) getNextPossibleThreads(threads []ProcThread) []ProcThread {
	possibleThreads := []ProcThread{}

	for i := 0; i < len(threads); i++ {
		if threads[i].GetStatus() == dara.Runnable {
			possibleThreads = append(possibleThreads, threads[i])
		}
	}
	return possibleThreads
}

func (e *Explorer) GetNextThread(threads []ProcThread) ProcThread {
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

func MountExplorer(logFilePath string, explorationStrategy Strategy) (*Explorer, error) {
	e := &Explorer{logFilePath : logFilePath, strategy : explorationStrategy}
	err := e.init()
	if err != nil {
		return nil, err
	}
	return e, nil
}
