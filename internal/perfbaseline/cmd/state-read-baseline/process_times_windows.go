//go:build windows

package main

import "os"

func processTimesMS(state *os.ProcessState) (float64, float64, bool) {
	return 0, 0, false
}
