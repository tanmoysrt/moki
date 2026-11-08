package main

import (
	"strconv"
	"syscall"
)

const detachedProcess = 0x00000008 // DETACHED_PROCESS: no console

// detached starts the child without a console, so it outlives the terminal.
func detached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: detachedProcess | syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}

func stopCommand(pid int) string {
	return "taskkill /PID " + strconv.Itoa(pid) + " /F"
}
