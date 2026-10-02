package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── NewSession ───────────────────────────────────────────────────────────────

func TestNewSession_UniqueIDs(t *testing.T) {
	s1 := NewSession("Chrome", false)
	s2 := NewSession("Chrome", false)
	if s1.SessionID == s2.SessionID {
		t.Errorf("NewSession generated duplicate IDs: %s", s1.SessionID)
	}
}

func TestNewSession_Fields(t *testing.T) {
	before := time.Now().Truncate(time.Second)
	s := NewSession("Brave", true)
	after := time.Now().Add(time.Second)

	if s.AppName != "Brave" {
		t.Errorf("AppName = %q; want %q", s.AppName, "Brave")
	}
	if !s.DryRun {
		t.Error("DryRun = false; want true")
	}
	if s.StartedAt.Before(before) || s.StartedAt.After(after) {
		t.Errorf("StartedAt %v out of expected range [%v, %v]", s.StartedAt, before, after)
	}
	if s.SessionID == "" {
		t.Error("SessionID is empty")
	}
}

// ─── LogSession ──────────────────────────────────────────────────────────────

// logToTempFile overrides the log path to a file inside dir so tests never
// touch the real ~/.cleanser_history.log.
func logToTempFile(t *testing.T) (logPath string, restore func()) {
	t.Helper()
	dir := t.TempDir()
	logPath = filepath.Join(dir, ".cleanser_history.log")

	origHome := os.Getenv("HOME")
	// Point HOME at the temp dir so resolveLogPath() picks up the temp file.
	if err := os.Setenv("HOME", dir); err != nil {
		t.Fatalf("Setenv HOME: %v", err)
	}
	restore = func() { os.Setenv("HOME", origHome) }
	return logPath, restore
}

func TestLogSession_CreatesFile(t *testing.T) {
	logPath, restore := logToTempFile(t)
	defer restore()

	s := NewSession("TestApp", false)
	if err := LogSession(s); err != nil {
		t.Fatalf("LogSession: %v", err)
	}

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file was not created")
	}
}

func TestLogSession_AppendOnly(t *testing.T) {
	_, restore := logToTempFile(t)
	defer restore()

	s1 := NewSession("Chrome", false)
	s1.Deleted = []string{"/path/to/cache"}

	s2 := NewSession("Brave", true)
	s2.Skipped = []string{"/path/to/prefs"}

	if err := LogSession(s1); err != nil {
		t.Fatalf("first LogSession: %v", err)
	}
	if err := LogSession(s2); err != nil {
		t.Fatalf("second LogSession: %v", err)
	}

	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".cleanser_history.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	content := string(data)

	// Both sessions must appear in the file.
	if !strings.Contains(content, s1.SessionID) {
		t.Errorf("log missing session 1 ID %s", s1.SessionID)
	}
	if !strings.Contains(content, s2.SessionID) {
		t.Errorf("log missing session 2 ID %s", s2.SessionID)
	}
	if !strings.Contains(content, "REMOVED: /path/to/cache") {
		t.Error("log missing REMOVED entry")
	}
	if !strings.Contains(content, "SKIPPED: /path/to/prefs") {
		t.Error("log missing SKIPPED entry")
	}
	if !strings.Contains(content, "DRY-RUN") {
		t.Error("log missing DRY-RUN marker")
	}
	if !strings.Contains(content, "LIVE") {
		t.Error("log missing LIVE marker")
	}
}

func TestLogSession_DryRunMode(t *testing.T) {
	_, restore := logToTempFile(t)
	defer restore()

	s := NewSession("Firefox", true)
	s.Skipped = []string{"/some/file"}

	if err := LogSession(s); err != nil {
		t.Fatalf("LogSession: %v", err)
	}

	home, _ := os.UserHomeDir()
	data, _ := os.ReadFile(filepath.Join(home, ".cleanser_history.log"))
	if !strings.Contains(string(data), "DRY-RUN") {
		t.Error("dry-run session not marked as DRY-RUN in log")
	}
}

func TestLogSession_Idempotent(t *testing.T) {
	// Calling LogSession multiple times must not overwrite previous entries.
	_, restore := logToTempFile(t)
	defer restore()

	const runs = 5
	var ids []string
	for i := 0; i < runs; i++ {
		s := NewSession("App", false)
		ids = append(ids, s.SessionID)
		if err := LogSession(s); err != nil {
			t.Fatalf("LogSession run %d: %v", i, err)
		}
	}

	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".cleanser_history.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	content := string(data)
	for _, id := range ids {
		if !strings.Contains(content, id) {
			t.Errorf("log missing session ID %s after %d runs", id, runs)
		}
	}
}

func TestLogSession_EmptyLists(t *testing.T) {
	// A session with no deleted or skipped paths must not panic.
	_, restore := logToTempFile(t)
	defer restore()

	s := NewSession("EmptyApp", false)
	if err := LogSession(s); err != nil {
		t.Fatalf("LogSession(empty): %v", err)
	}
}

// ─── LogPath ─────────────────────────────────────────────────────────────────

func TestLogPath_ContainsFileName(t *testing.T) {
	p := LogPath()
	if !strings.HasSuffix(p, logFileName) {
		t.Errorf("LogPath = %q; want suffix %q", p, logFileName)
	}
}
