package main

import (
	"bufio"
	"dara"
	"fmt"
	"github.com/DARA-Project/GoDist-Scheduler/common"
	"log"
	"os"
	"strings"
)

func read_blocks_file(filename string) (*map[string]bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	allBlocks := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		block := strings.TrimSpace(scanner.Text())
		if block != "" {
			allBlocks[block] = true
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	return &allBlocks, nil
}

func print_coverage(schedule *dara.Schedule, blocks *map[string]bool) {
	covered_blocks := make(map[string]uint64)
	for _, covEvent := range schedule.CovEvents {
		for block, count := range covEvent.CoverageInfo {
			if v, ok := covered_blocks[block]; ok {
				covered_blocks[block] = v + count
			} else {
				covered_blocks[block] = count
			}
		}
	}
	fmt.Println("Total # of blocks in source code:", len(*blocks))
	fmt.Println("Total # of blocks covered:", len(covered_blocks))
	fmt.Println("Total # of blocks uncovered:", len(*blocks) - len(covered_blocks))
	fmt.Println("Covered Block \t Frequency")
	for block, count := range covered_blocks {
		fmt.Println(block, "\t", count)
	}
	fmt.Println("Uncovered Blocks:")
	for block, _ := range *blocks {
		if _, ok := covered_blocks[block]; !ok {
			fmt.Println(block)
		}
	}
}

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: go run cov_report.go <schedule_filename> <blocks_filename>")
	}
	schedule_filename := os.Args[1]
	blocks_filename := os.Args[2]

	schedule, err := common.ReadSchedule(schedule_filename)
	if err != nil {
		log.Fatal(err)
	}
	blocks, err := read_blocks_file(blocks_filename)
	if err != nil {
		log.Fatal(err)
	}
	print_coverage(schedule, blocks)
}