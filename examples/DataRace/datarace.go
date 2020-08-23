package main

import (
	"sync"
	"fmt"
	"runtime"
)

func main() {
	runtime.ReportBlockCoverage("../examples/DataRace/datarace.go:9:12")
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		runtime.ReportBlockCoverage("../examples/DataRace/datarace.go:12:13")
		go func() {
			runtime.ReportBlockCoverage("../examples/DataRace/datarace.go:13:17")
			fmt.Println(i)
			runtime.DaraLog("Child", "child_i", i)
			wg.Done()
		}()
	}
	runtime.ReportBlockCoverage("../examples/DataRace/datarace.go:19:19")
	wg.Wait()
}
