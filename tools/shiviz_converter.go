package main

import (
	"dara"
	"github.com/DARA-Project/GoDist-Scheduler/common"
	"github.com/DistributedClocks/GoVector/govec/vclock"
	"log"
	"os"
	"strconv"
)

func initialize_shiviz_file(filename string) (*os.File, error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	n, err := f.WriteString("(?<host>\\S*) (?<clock>{.*})\\n(?<event>.*)\n\n")
	if err != nil {
		f.Close()
		return nil, err
	}
	log.Println("Initialized shiviz log file by writing", n, "bytes")
	return f, nil
}

func uniqueThreadID(procID int, goid int) string {
	return strconv.Itoa(procID) + ":" + strconv.Itoa(goid)
}

func parse_schedule(schedule *dara.Schedule, filename string) error {
	f, err := initialize_shiviz_file(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	clocks := make(map[string]*vclock.VClock)
	routine_names := make(map[string]string)
	coverage_map := make(map[int]string)
	for _, covEvent := range schedule.CovEvents {
		coverage_map[covEvent.EventIndex] = common.CoverageString(&covEvent)
	}
	curr_running_routine := uniqueThreadID(1, 1)
	for idx, event := range schedule.LogEvents {
		var coverage string
		if v, ok := coverage_map[idx]; ok {
			coverage = v
		}
		goroutine_string := common.GoRoutineNameString(event.P, event.G)
		uniqueID := uniqueThreadID(event.P, event.G.Gid)
		if _, ok := routine_names[uniqueID]; !ok {
			routine_names[uniqueID] = goroutine_string
		}
		if vc, ok := clocks[uniqueID]; !ok {
			clock := vclock.New()
			clocks[uniqueID] = &clock
			clock.Tick(goroutine_string)
			// Tick the goroutine that is getting off and merge the vector
			// clock of the old goroutine with the newly launched goroutine
			if event.Type == dara.SCHED_EVENT && curr_running_routine != uniqueID {
				prev_routine_clock := clocks[curr_running_routine]
				prev_routine_name := routine_names[curr_running_routine]
				prev_routine_clock.Tick(prev_routine_name)
				eventString := common.ConciseEventString(&event)
				if coverage != "" {
					eventString += "; Coverage: " + coverage
				}
				n, err := f.WriteString(prev_routine_name + " " + prev_routine_clock.ReturnVCString() + "\n" +
					eventString + "\n")
				if err != nil {
					return err
				}
				clock.Merge(*clocks[curr_running_routine])
				log.Printf("Wrote %d bytes\n", n)
			}
			eventString := common.ConciseEventString(&event)
			if coverage != "" {
				eventString += "; Coverage: " + coverage
			}
			n, err := f.WriteString(goroutine_string + " " + clock.ReturnVCString() + "\n" +
				eventString + "\n")
			if err != nil {
				return err
			}
			log.Printf("Wrote %d bytes\n", n)
		} else {
			vc.Tick(goroutine_string)
			// Tick the goroutine that is getting off and merge the vector
			// clock of the old goroutine with the newly launched goroutine
			if event.Type == dara.SCHED_EVENT && curr_running_routine != uniqueID {
				prev_routine_clock := clocks[curr_running_routine]
				prev_routine_name := routine_names[curr_running_routine]
				prev_routine_clock.Tick(prev_routine_name)
				eventString := common.ConciseEventString(&event)
				if coverage != "" {
					eventString += "; Coverage: " + coverage
				}
				n, err := f.WriteString(prev_routine_name + " " + prev_routine_clock.ReturnVCString() + "\n" +
					eventString + "\n")
				if err != nil {
					return err
				}
				vc.Merge(*clocks[curr_running_routine])
				log.Printf("Wrote %d bytes\n", n)
			}
			eventString := common.ConciseEventString(&event)
			if coverage != "" {
				eventString += "; Coverage: " + coverage
			}
			n, err := f.WriteString(goroutine_string + " " + vc.ReturnVCString() + "\n" +
				eventString + "\n")
			if err != nil {
				return err
			}
			log.Printf("Wrote %d bytes\n", n)
		}
		if event.Type != dara.THREAD_EVENT {
			curr_running_routine = uniqueID
		}
	}
	return nil
}

func main() {
	if len(os.Args) != 3 {
		log.Fatal("Usage: go run shiviz_converter.go <schedule_filename> <shiviz_filename>")
	}
	filename := os.Args[1]
	shiviz_filename := os.Args[2]
	schedule, err := common.ReadSchedule(filename)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Length of the schedule is ", len(schedule.LogEvents))
	err = parse_schedule(schedule, shiviz_filename)
	if err != nil {
		log.Fatal(err)
	}
}
