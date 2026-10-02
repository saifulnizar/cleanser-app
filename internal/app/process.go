package app

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// IsRunning checks whether any process matching appName is currently active.
// It returns true plus the list of matching PIDs.
func IsRunning(appName string) (bool, []int, error) {
	// pgrep performs a case-insensitive, partial match against process names.
	out, err := exec.Command("pgrep", "-il", appName).Output()
	if err != nil {
		// Exit code 1 from pgrep means no match — not an error condition.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("process: pgrep: %w", err)
	}

	pids := parsePIDs(string(out))
	return len(pids) > 0, pids, nil
}

// TerminateApp sends SIGTERM to each PID, waits up to 3 seconds, then sends SIGKILL
// if the process is still alive. This is a best-effort operation; errors for
// individual PIDs are collected and returned as a combined error.
func TerminateApp(pids []int) error {
	var errs []string

	for _, pid := range pids {
		// Step 1: graceful termination.
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			errs = append(errs, fmt.Sprintf("SIGTERM pid %d: %v", pid, err))
			continue
		}

		// Step 2: wait up to 3 seconds for the process to exit.
		if alive := waitForExit(pid, 3*time.Second); alive {
			// Step 3: force kill if still running.
			if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
				errs = append(errs, fmt.Sprintf("SIGKILL pid %d: %v", pid, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("process: terminate: %s", strings.Join(errs, "; "))
	}
	return nil
}

// waitForExit polls the process every 200 ms until it exits or the timeout elapses.
// It returns true if the process is still alive after the timeout.
func waitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Signal 0 checks existence without sending a real signal.
		if err := syscall.Kill(pid, 0); err != nil {
			// Process no longer exists.
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
	return true
}

// parsePIDs extracts integer PIDs from pgrep output lines (format: "PID procname").
func parsePIDs(output string) []int {
	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// pgrep -l format: "<pid> <name>"
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		pid, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}
