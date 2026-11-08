//go:build !windows

package main

import (
	"strconv"
	"syscall"
)

// detached starts the child in its own session, so it outlives the terminal.
func detached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

func stopCommand(pid int) string {
	return "kill " + strconv.Itoa(pid)
}
