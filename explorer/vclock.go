package explorer

import (
	"math"
	"log"
	"dara"
)

type TimeEventType int

const (
	Sleep TimeEventType = iota
	Timer
)

type TimeEvent struct {
	Type TimeEventType
	When uint64 // When do we need to perform the time related action
	Period uint64 // if it is a periodic event then it must be fired, well, periodically.
	GoRoutineID int64 // if it is a sleeping event, we need to know which goroutine to wake up!
	TimerID int64 // Need to know which timer to fire off
}

type VirtualClock struct {
	CurrentTick uint64
	WaitingQ map[int]*TimeEvent // The queues are mainly arrays
	ReadyQ []*TimeEvent
	sleepingRoutines map[int64]bool
}

func NewClock() *VirtualClock {
	return &VirtualClock{CurrentTick: 0, WaitingQ: make(map[int]*TimeEvent), ReadyQ: []*TimeEvent{}, sleepingRoutines: make(map[int64]bool)}
}

func (v *VirtualClock) getNextEventFromReady() *TimeEvent {
	// Need to pick something from the readyq
	// and then remove the element from the readyq
	dprint(dara.DEBUG, func() {log.Println("[VClock] Length of ReadyQ", len(v.ReadyQ), ", length of waitingQ", len(v.WaitingQ))})
	lowestIndex := -1
	var lowestVal uint64
	lowestVal = math.MaxUint64
	var timeEvent *TimeEvent
	for i, element := range v.ReadyQ {
		if element.Type == Sleep {
			continue
		}
		if element.When <= lowestVal {
			timeEvent = element
			lowestIndex = i
		}
	}
	if lowestIndex == -1 {
		dprint(dara.DEBUG, func() {log.Println("[VClock] Why is my lowest index -1")})
	}
	v.ReadyQ[lowestIndex] = v.ReadyQ[len(v.ReadyQ) - 1]
	v.ReadyQ[len(v.ReadyQ) - 1] = &TimeEvent{}
	v.ReadyQ = v.ReadyQ[:len(v.ReadyQ) - 1]
	return timeEvent
}

func (v *VirtualClock) GetNextEvent() *TimeEvent {
	if len(v.ReadyQ) != 0 {
		return v.getNextEventFromReady()
	}
	// Otherwise pick the earliest from the WaitingQ
	var earliestTick uint64
	earliestTick = math.MaxUint64
	dprint(dara.DEBUG, func() {log.Println("Number of elements on WaitingQ", len(v.WaitingQ))})
	for _, event := range v.WaitingQ {
		if event.Type == Sleep {
			continue
		}
		if event.When <= earliestTick {
			earliestTick = event.When
		}
	}
	if earliestTick == math.MaxUint64 {
		dprint(dara.DEBUG, func() {log.Println("WaitingQ only had sleeping events")})
	}
	// And fastforward the clock to the ticks.
	v.SetTick(earliestTick)
	// And then return that from the ready queue
	if len(v.ReadyQ) == 0 {
		return nil // Return nil to indicate that no action was found but we did wake up something!
	}
	return v.getNextEventFromReady()
}

func (v *VirtualClock) SetTick(amount uint64) {
	v.CurrentTick = amount
	v.updateQs()
}

func (v *VirtualClock) Tick(amount uint64) {
	v.CurrentTick += amount
	v.updateQs()
}

func (v *VirtualClock) IsSleeping(gid int) bool {
	if val, ok := v.sleepingRoutines[int64(gid)]; ok {
		return val
	}
	return false
}

func (v *VirtualClock) AddTimerEvent(eventID int, eventType TimeEventType, when uint64, period uint64, gid int64, timerID int64) {
	event := TimeEvent{Type: eventType, When: v.CurrentTick + when, Period: period, GoRoutineID: gid, TimerID: timerID}
	v.WaitingQ[eventID] = &event
	if eventType == Sleep {
		v.sleepingRoutines[gid] = true
	}
}

func (v *VirtualClock) updateQs() {
	for id, event := range v.WaitingQ {
		if event.When <= v.CurrentTick {
			if event.Type == Sleep {
				delete(v.sleepingRoutines, event.GoRoutineID)
			} else {
				v.ReadyQ = append(v.ReadyQ, event)
			}
			if event.Period != 0 {
				v.WaitingQ[id] = &TimeEvent{Type: event.Type, When: v.CurrentTick + event.Period, Period: event.Period, TimerID: event.TimerID, GoRoutineID: event.GoRoutineID}
			} else {
				delete(v.WaitingQ, id)
			}
		}
	}
}