//go:build !windows

package main

import (
	"os"
	"syscall"
)

func processTimesMS(state *os.ProcessState) (float64, float64, bool) {
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0, 0, false
	}
	return durationMS(usage.Utime), durationMS(usage.Stime), true
}

func durationMS(tv syscall.Timeval) float64 {
	return float64(tv.Sec)*1000 + float64(tv.Usec)/1000
}
