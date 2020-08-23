package main

import (
	"testing"
	"runtime"
)

func BenchmarkDaraOnCheck(b *testing.B) {
	for i := 0; i < b.N; i++ {
		runtime.Is_dara_profiling_on()
	}
}
