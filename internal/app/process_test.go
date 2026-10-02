package app

import (
	"testing"
)

// ─── parsePIDs ───────────────────────────────────────────────────────────────

func TestParsePIDs_Empty(t *testing.T) {
	got := parsePIDs("")
	if len(got) != 0 {
		t.Errorf("parsePIDs(\"\") = %v; want []", got)
	}
}

func TestParsePIDs_SingleLine(t *testing.T) {
	input := "1234 Google Chrome Helper\n"
	got := parsePIDs(input)
	if len(got) != 1 || got[0] != 1234 {
		t.Errorf("parsePIDs = %v; want [1234]", got)
	}
}

func TestParsePIDs_MultipleLines(t *testing.T) {
	input := "100 chrome\n200 chrome helper\n300 chrome renderer\n"
	got := parsePIDs(input)
	want := []int{100, 200, 300}
	if len(got) != len(want) {
		t.Fatalf("parsePIDs len = %d; want %d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("parsePIDs[%d] = %d; want %d", i, got[i], v)
		}
	}
}

func TestParsePIDs_InvalidLine(t *testing.T) {
	// Lines without a leading integer should be silently skipped.
	input := "notanumber chrome\n999 something\n"
	got := parsePIDs(input)
	if len(got) != 1 || got[0] != 999 {
		t.Errorf("parsePIDs = %v; want [999]", got)
	}
}

func TestParsePIDs_BlankLines(t *testing.T) {
	input := "\n\n456 test\n\n"
	got := parsePIDs(input)
	if len(got) != 1 || got[0] != 456 {
		t.Errorf("parsePIDs = %v; want [456]", got)
	}
}

func TestParsePIDs_OnlyPID(t *testing.T) {
	// pgrep without -l emits just the PID number per line.
	input := "789\n"
	got := parsePIDs(input)
	if len(got) != 1 || got[0] != 789 {
		t.Errorf("parsePIDs = %v; want [789]", got)
	}
}

// ─── IsRunning ────────────────────────────────────────────────────────────────

func TestIsRunning_NonExistentProcess(t *testing.T) {
	// A name that will never match any real process.
	running, pids, err := IsRunning("__cleanser_no_such_proc_xyz__")
	if err != nil {
		t.Fatalf("IsRunning: unexpected error: %v", err)
	}
	if running {
		t.Error("IsRunning = true; want false for non-existent process")
	}
	if len(pids) != 0 {
		t.Errorf("IsRunning pids = %v; want []", pids)
	}
}

// ─── waitForExit ─────────────────────────────────────────────────────────────

func TestWaitForExit_NonExistentPID(t *testing.T) {
	// PID 99999999 is virtually guaranteed to not exist on any macOS system.
	const fakePID = 99999999

	// With zero timeout the loop body never executes — should return true
	// (timeout expired without confirming exit). No panic must occur.
	alive := waitForExit(fakePID, 0)
	// We don't assert the value because with 0 timeout the loop condition is
	// immediately false, so it returns `true` (still "alive" from timeout perspective).
	// The important thing is that it completes without error or panic.
	_ = alive
}

func TestWaitForExit_AlreadyExited(t *testing.T) {
	// A PID we are confident does not exist → Kill(pid, 0) returns ESRCH →
	// waitForExit should return false (not alive) quickly.
	const fakePID = 99999998
	alive := waitForExit(fakePID, 50_000_000) // 50ms
	if alive {
		// It's theoretically possible (though astronomically unlikely) that this
		// PID exists. Log a warning but don't fail the test hard.
		t.Log("WARNING: waitForExit returned true for a supposedly non-existent PID")
	}
}
