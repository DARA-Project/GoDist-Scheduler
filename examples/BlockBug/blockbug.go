package main

import "runtime"

func finishReq(timeout time.Duration) ob {
	runtime.ReportBlockCoverage("../examples/BlockBug/blockbug.go:3:5")
	ch := make(chan ob)
	go func() {
		runtime.ReportBlockCoverage("../examples/BlockBug/blockbug.go:5:8")
		result := fn()
		ch <- result
	}()
	runtime.ReportBlockCoverage("../examples/BlockBug/blockbug.go:9:9")
	select {
	case result := <-ch:
		runtime.ReportBlockCoverage("../examples/BlockBug/blockbug.go:10:11")
		return result
	case <-time.After(timeout):
		runtime.ReportBlockCoverage("../examples/BlockBug/blockbug.go:12:13")
		return nil
	}
}
