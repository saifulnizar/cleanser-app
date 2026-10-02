package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const logFileName = ".cleanser_history.log"

// Session holds the data for one cleanser run to be persisted in the audit log.
type Session struct {
	// SessionID is a unique identifier for this run (UUID v4).
	SessionID string
	// AppName is the human-readable application name.
	AppName string
	// Deleted is the list of paths that were successfully removed.
	Deleted []string
	// Skipped is the list of paths that were not present (idempotent skip).
	Skipped []string
	// DryRun indicates this was a simulation — nothing was actually deleted.
	DryRun bool
	// StartedAt is the timestamp when the session began.
	StartedAt time.Time
}

// NewSession creates a new Session with a unique ID and the current timestamp.
func NewSession(appName string, dryRun bool) Session {
	return Session{
		SessionID: uuid.New().String(),
		AppName:   appName,
		DryRun:    dryRun,
		StartedAt: time.Now(),
	}
}

// LogSession appends the session result to ~/.cleanser_history.log.
// It is safe to call multiple times (append-only, never overwrites).
func LogSession(session Session) error {
	logPath, err := resolveLogPath()
	if err != nil {
		return fmt.Errorf("logger: resolve log path: %w", err)
	}

	// Open in append-only mode; create if it does not exist.
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("logger: open log file: %w", err)
	}
	defer f.Close()

	ts := session.StartedAt.UTC().Format(time.RFC3339)
	mode := "LIVE"
	if session.DryRun {
		mode = "DRY-RUN"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] [%s] [%s] APP=%q\n", ts, session.SessionID, mode, session.AppName))

	for _, p := range session.Deleted {
		sb.WriteString(fmt.Sprintf("  REMOVED: %s\n", p))
	}
	for _, p := range session.Skipped {
		sb.WriteString(fmt.Sprintf("  SKIPPED: %s\n", p))
	}

	if _, err := f.WriteString(sb.String()); err != nil {
		return fmt.Errorf("logger: write log: %w", err)
	}
	return nil
}

// resolveLogPath returns the absolute path to the log file under the user's home directory.
func resolveLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, logFileName), nil
}

// LogPath returns the resolved log file path for display purposes.
func LogPath() string {
	p, _ := resolveLogPath()
	return p
}
