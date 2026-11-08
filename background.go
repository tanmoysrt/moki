package main

// moki --bg: start moki again without --bg, detached from the terminal, and wait
// until it answers. Its output goes to moki.log in the data folder.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func startBackground(cfg Config) error {
	if running(cfg.localURL()) {
		printRunning(cfg)
		return nil
	}
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(cfg.Dir, "moki.log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, withoutBg(os.Args[1:])...)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = detached()
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := waitUntilUp(cmd, cfg.localURL()); err != nil {
		return fmt.Errorf("%v%s. See the log: %s", err, lastLine(logPath), tildePath(logPath))
	}
	printBackground(cfg, cmd.Process.Pid, logPath)
	return nil
}

// waitUntilUp waits for /health, and fails when the child stops first.
func waitUntilUp(cmd *exec.Cmd, url string) error {
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case err := <-exited:
			if err == nil {
				err = errors.New("it stopped")
			}
			return fmt.Errorf("did not start (%v)", err)
		case <-deadline:
			return errors.New("did not answer in 10 seconds")
		case <-time.After(100 * time.Millisecond):
			if running(url) {
				return nil
			}
		}
	}
}

// lastLine is the last line of the log, which says why moki stopped.
func lastLine(path string) string {
	b, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if last := strings.TrimPrefix(strings.TrimSpace(lines[len(lines)-1]), "moki: "); last != "" {
		return ": " + last
	}
	return ""
}

// withoutBg drops --bg from the arguments, in any of its forms.
func withoutBg(args []string) []string {
	var out []string
	for _, a := range args {
		name := strings.TrimLeft(a, "-")
		if strings.HasPrefix(a, "-") && (name == "bg" || strings.HasPrefix(name, "bg=")) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func printBackground(cfg Config, pid int, logPath string) {
	printLogo()
	row("Open", cyan(cfg.URL))
	row("Log", tildePath(logPath))
	row("Stop", stopCommand(pid))
	fmt.Println()
	fmt.Println("  " + dim("moki runs in the background."))
	fmt.Println()
}
