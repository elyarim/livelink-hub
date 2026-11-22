//go:build !windows

package main

import (
	"errors"
	"time"
)

const shutdownDelay = 20 * time.Second

func shutdownAt() int64 { return 0 }

func requestShutdown() error {
	return errors.New("remote host shutdown is only supported on Windows desktop")
}

func cancelShutdown() error { return nil }
