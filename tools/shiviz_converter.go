package main

import (
	"dara"
	"encoding/json"
	"github.com/DARA-Project/GoDist-Scheduler/common"
	"github.com/DistributedClocks/GoVector/govec/vclock"
	"log"
	"os"
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

func parse_schedule(schedule *dara.Schedule, filename string) error {
	f, err := initialize_shiviz_file(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	clocks := make(map[int]*vclock.VClock)
	routine_names := make(map[int]string)
	curr_running_routine := 1
	for _, event := range *schedule {
		goroutine_string := common.GoRoutineNameString(event.P, event.G)
		if _, ok := routine_names[event.G.Gid]; !ok {
			routine_names[event.G.Gid] = goroutine_string
		}
		if vc, ok := clocks[event.G.Gid]; !ok {
			clock := vclock.New()
			clocks[event.G.Gid] = &clock
			clock.Tick(goroutine_string)
			// Tick the goroutine that is getting off and merge the vector
			// clock of the old goroutine with the newly launched goroutine
			if event.Type == dara.SCHED_EVENT && curr_running_routine != event.G.Gid {
				prev_routine_clock := clocks[curr_running_routine]
				prev_routine_name := routine_names[curr_running_routine]
				prev_routine_clock.Tick(prev_routine_name)
				n, err := f.WriteString(prev_routine_name + " " + prev_routine_clock.ReturnVCString() + "\n" +
					common.ConciseEventString(&event) + "\n")
				if err != nil {
					return err
				}
				clock.Merge(*clocks[curr_running_routine])
				log.Printf("Wrote %d bytes\n", n)
			}
			n, err := f.WriteString(goroutine_string + " " + clock.ReturnVCString() + "\n" +
				common.ConciseEventString(&event) + "\n")
			if err != nil {
				return err
			}
			log.Printf("Wrote %d bytes\n", n)
		} else {
			vc.Tick(goroutine_string)
			// Tick the goroutine that is getting off and merge the vector
			// clock of the old goroutine with the newly launched goroutine
			if event.Type == dara.SCHED_EVENT && curr_running_routine != event.G.Gid {
				prev_routine_clock := clocks[curr_running_routine]
				prev_routine_name := routine_names[curr_running_routine]
				prev_routine_clock.Tick(prev_routine_name)
				n, err := f.WriteString(prev_routine_name + " " + prev_routine_clock.ReturnVCString() + "\n" +
					common.ConciseEventString(&event) + "\n")
				if err != nil {
					return err
				}
				vc.Merge(*clocks[curr_running_routine])
				log.Printf("Wrote %d bytes\n", n)
			}
			n, err := f.WriteString(goroutine_string + " " + vc.ReturnVCString() + "\n" +
				common.ConciseEventString(&event) + "\n")
			if err != nil {
				return err
			}
			log.Printf("Wrote %d bytes\n", n)
		}
		if event.Type != dara.THREAD_EVENT {
			curr_running_routine = event.G.Gid
		}
	}
	return nil
}

func read_schedule(filename string) (*dara.Schedule, error) {
	var schedule dara.Schedule
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(f)
	err = dec.Decode(&schedule)
	if err != nil {
		return nil, err
	}
	return &schedule, nil
}

func main() {
	if len(os.Args) != 3 {
		log.Fatal("Usage: go run shiviz_converter.go <schedule_filename> <shiviz_filename>")
	}
	filename := os.Args[1]
	shiviz_filename := os.Args[2]
	schedule, err := read_schedule(filename)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Length of the schedule is ", len(*schedule))
	err = parse_schedule(schedule, shiviz_filename)
	if err != nil {
		log.Fatal(err)
	}
}
