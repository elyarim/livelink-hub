//go:build !windows

package main

func setHighPriority() {
	// On Linux, process scheduling / nice / CAP_SYS_NICE is managed by systemd or Docker.
}
