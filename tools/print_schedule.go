package main

import (
	"dara"
	"fmt"
	"github.com/DARA-Project/GoDist-Scheduler/common"
	"log"
	"os"
)

func print_schedule(schedule *dara.Schedule) {
	for _, event := range schedule.LogEvents {
		fmt.Println("Process", event.P, " Event:", common.ConciseEventString(&event))
	}
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go print_schedule.go <schedule_filename>")
	}
	filename := os.Args[1]
	schedule, err := common.ReadSchedule(filename)
	if err != nil {
		log.Fatal(err)
	}
	print_schedule(schedule)
}