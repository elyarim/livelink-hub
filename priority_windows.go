//go:build windows

package main

import (
	"log"

	"golang.org/x/sys/windows"
)

// setHighPriority raises process priority on Windows so background OS tasks
// do not introduce jitter into live SRT packet forwarding.
func setHighPriority() {
	h := windows.CurrentProcess()
	if err := windows.SetPriorityClass(h, windows.HIGH_PRIORITY_CLASS); err != nil {
		log.Printf("process priority: could not set HIGH_PRIORITY_CLASS: %v", err)
	}
}
