package deleter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// dangerousPaths is a hard-coded guard list. Deleting any of these paths would
// be catastrophic. Any requested path that exactly matches or is a direct child
// of these paths will be refused.
var dangerousPaths = []string{
	"/",
	"/Applications",
	"/System",
	"/Users",
	"/Library",
	"/private",
	"/usr",
	"/bin",
	"/sbin",
	"/etc",
	"/var",
}

// Result holds the outcome of a deletion session.
type Result struct {
	// Deleted contains paths that were successfully removed.
	Deleted []string
	// Skipped contains paths that did not exist (idempotent skip).
	Skipped []string
	// Errors contains per-path errors that did not abort the overall operation.
	Errors []error
}

// Delete removes all paths in the list. It is idempotent: if a path no longer
// exists it is silently skipped. If os.RemoveAll fails due to a permission error,
// it transparently escalates to AppleScript (native macOS auth dialog).
//
// When dryRun is true, nothing is deleted and all paths are added to Skipped.
// The context is checked before each deletion so Ctrl+C aborts cleanly.
func Delete(ctx context.Context, paths []string, dryRun bool) (*Result, error) {
	result := &Result{}

	for _, p := range paths {
		// Honour context cancellation between each file.
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		// Safety guard — never delete dangerous top-level paths.
		if err := guardPath(p); err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}

		// Idempotency: skip if the path no longer exists.
		if _, err := os.Lstat(p); os.IsNotExist(err) {
			result.Skipped = append(result.Skipped, p)
			continue
		}

		if dryRun {
			// Dry-run: record as skipped (nothing is actually deleted).
			result.Skipped = append(result.Skipped, p)
			continue
		}

		if err := deletePath(p); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("delete %s: %w", p, err))
		} else {
			result.Deleted = append(result.Deleted, p)
		}
	}

	return result, nil
}

// deletePath attempts os.RemoveAll first; on permission error it escalates to
// AppleScript so the native macOS authentication dialog (Touch ID / password)
// is shown to the user.
func deletePath(path string) error {
	if err := os.RemoveAll(path); err == nil {
		return nil
	} else if !isPermissionError(err) {
		return err
	}

	// Escalate: use osascript for privileged deletion.
	return deleteWithAppleScript(path)
}

// deleteWithAppleScript calls the native macOS authentication dialog and then
// executes rm -rf with administrator privileges.
func deleteWithAppleScript(path string) error {
	// Escape single quotes in the path to safely embed it in the shell command.
	safePath := strings.ReplaceAll(path, "'", "'\\''")

	script := fmt.Sprintf(
		`do shell script "rm -rf '%s'" with administrator privileges`,
		safePath,
	)

	cmd := exec.Command("osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("osascript delete failed: %w — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// guardPath returns an error if path is or is a direct child of a dangerous path.
func guardPath(path string) error {
	// Refuse empty paths first.
	if path == "" {
		return errors.New("deleter: refusing to delete empty path")
	}

	// filepath.Clean normalises "//", trailing slashes, and ".." components.
	// This means "/" stays "/" and "/usr/" becomes "/usr".
	clean := filepath.Clean(path)

	for _, danger := range dangerousPaths {
		if clean == danger {
			return fmt.Errorf("deleter: refusing to delete protected path: %s", path)
		}
	}

	return nil
}

// isPermissionError reports whether the error (possibly wrapped) is a
// permission-denied OS error.
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, os.ErrPermission)
}
