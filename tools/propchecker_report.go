package main

import (
	"dara"
	"fmt"
	"github.com/DARA-Project/GoDist-Scheduler/common"
	"log"
	"os"
)

func print_propcheck_report(schedule *dara.Schedule) {
	fmt.Println("Total number of property checks:", len(schedule.PropEvents))
	for _, event := range schedule.PropEvents {
		fmt.Println("Index:", event.EventIndex, common.PropCheckString(&event))
	}
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: go run cov_report.go <schedule_filename>")
	}
	schedule_filename := os.Args[1]

	schedule, err := common.ReadSchedule(schedule_filename)
	if err != nil {
		log.Fatal(err)
	}
	print_propcheck_report(schedule)
}