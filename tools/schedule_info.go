package main

import (
	"dara"
	"fmt"
	"github.com/DARA-Project/GoDist-Scheduler/common"
	"log"
	"os"
)

func stats(schedule *dara.Schedule) {
	event_stats := make(map[int]int)
	for _, event := range schedule.LogEvents {
		event_stats[event.Type] = event_stats[event.Type] + 1
	}
	for k, v := range event_stats {
		log.Println(common.EventTypeString(k), " Events :", v)
	}
}

func get_num_schedule(schedule *dara.Schedule) int {
	event_stats := make(map[int]int)
	for _, event := range schedule.LogEvents {
		event_stats[event.Type] = event_stats[event.Type] + 1
	}
	return event_stats[dara.SCHED_EVENT]
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run schedule_info.go <schedule_filename>")
	}
	filename := os.Args[1]
	schedule, err := common.ReadSchedule(filename)
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 2 && os.Args[2] == "scale" {
		num_schedule := get_num_schedule(schedule)
		fmt.Printf("%d,%d\n", len(schedule.LogEvents), num_schedule)
	} else {
		log.Println("SCHEDULE LENGTH :", len(schedule.LogEvents))
		stats(schedule)
	}
}
