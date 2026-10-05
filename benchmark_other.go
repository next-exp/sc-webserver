//go:build !windows

package main

import (
	"fmt"
	"time"
)

func processCPUTime() (time.Duration, error) {
	return 0, fmt.Errorf("capture benchmarking requires Windows")
}
