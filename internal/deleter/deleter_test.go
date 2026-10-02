package deleter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── guardPath ───────────────────────────────────────────────────────────────

func TestGuardPath_DangerousPaths(t *testing.T) {
	dangerous := []string{
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
		// With trailing slash — should also be blocked.
		"/usr/",
	}

	for _, p := range dangerous {
		t.Run(p, func(t *testing.T) {
			err := guardPath(p)
			if err == nil {
				t.Errorf("guardPath(%q) = nil; want error", p)
			}
		})
	}
}

func TestGuardPath_EmptyPath(t *testing.T) {
	err := guardPath("")
	if err == nil {
		t.Error("guardPath(\"\") = nil; want error")
	}
}

func TestGuardPath_SafePaths(t *testing.T) {
	safe := []string{
		"/Users/bob/Library/Caches/com.google.Chrome",
		"/Users/bob/Library/Application Support/Google/Chrome",
		"/tmp/test_file.txt",
	}

	for _, p := range safe {
		t.Run(p, func(t *testing.T) {
			err := guardPath(p)
			if err != nil {
				t.Errorf("guardPath(%q) = %v; want nil", p, err)
			}
		})
	}
}

// ─── isPermissionError ───────────────────────────────────────────────────────

func TestIsPermissionError_Nil(t *testing.T) {
	if isPermissionError(nil) {
		t.Error("isPermissionError(nil) = true; want false")
	}
}

func TestIsPermissionError_PermissionDenied(t *testing.T) {
	// Create a read-only file and try to remove it — this is the simplest way
	// to obtain a real os.ErrPermission on most systems.
	// Instead, we synthesise it directly via os.ErrPermission for speed.
	err := &os.PathError{Op: "remove", Path: "/x", Err: os.ErrPermission}
	if !isPermissionError(err) {
		t.Errorf("isPermissionError(permission error) = false; want true")
	}
}

func TestIsPermissionError_OtherError(t *testing.T) {
	err := &os.PathError{Op: "remove", Path: "/x", Err: os.ErrNotExist}
	if isPermissionError(err) {
		t.Errorf("isPermissionError(ErrNotExist) = true; want false")
	}
}

// ─── Delete — idempotency ─────────────────────────────────────────────────────

func TestDelete_IdempotentNonExistent(t *testing.T) {
	ctx := context.Background()
	// Path that does not exist — should be silently skipped (idempotent).
	result, err := Delete(ctx, []string{"/tmp/__cleanser_no_such_file_xyz__"}, false)
	if err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}
	if len(result.Deleted) != 0 {
		t.Errorf("Deleted = %v; want empty", result.Deleted)
	}
	if len(result.Skipped) != 1 {
		t.Errorf("Skipped = %v; want 1 entry", result.Skipped)
	}
}

func TestDelete_IdempotentCalledTwice(t *testing.T) {
	// Create a real temp file, delete it, then call Delete again on the same path.
	dir := t.TempDir()
	f := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// First call — should delete.
	r1, err := Delete(ctx, []string{f}, false)
	if err != nil {
		t.Fatalf("first Delete: %v", err)
	}
	if len(r1.Deleted) != 1 {
		t.Errorf("first Delete: Deleted = %v; want 1 entry", r1.Deleted)
	}

	// Second call on same path — file no longer exists → Skipped.
	r2, err := Delete(ctx, []string{f}, false)
	if err != nil {
		t.Fatalf("second Delete: %v", err)
	}
	if len(r2.Skipped) != 1 {
		t.Errorf("second Delete: Skipped = %v; want 1 entry", r2.Skipped)
	}
	if len(r2.Deleted) != 0 {
		t.Errorf("second Delete: Deleted = %v; want empty", r2.Deleted)
	}
}

// ─── Delete — dry-run ────────────────────────────────────────────────────────

func TestDelete_DryRun(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "keep_me.txt")
	if err := os.WriteFile(f, []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	result, err := Delete(ctx, []string{f}, true /* dryRun */)
	if err != nil {
		t.Fatalf("Delete dry-run: %v", err)
	}

	// File must still exist.
	if _, statErr := os.Stat(f); os.IsNotExist(statErr) {
		t.Error("dry-run: file was deleted but should have been preserved")
	}

	// Result must report it as Skipped (not Deleted).
	if len(result.Deleted) != 0 {
		t.Errorf("dry-run: Deleted = %v; want empty", result.Deleted)
	}
	if len(result.Skipped) != 1 {
		t.Errorf("dry-run: Skipped = %v; want 1 entry", result.Skipped)
	}
}

// ─── Delete — guard blocks dangerous paths ────────────────────────────────────

func TestDelete_BlocksDangerousPath(t *testing.T) {
	ctx := context.Background()
	result, err := Delete(ctx, []string{"/usr"}, false)
	if err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Error("Delete(/usr): expected guard error, got none")
	}
	if !strings.Contains(result.Errors[0].Error(), "protected path") {
		t.Errorf("error message = %q; want 'protected path'", result.Errors[0].Error())
	}
}

// ─── Delete — partial failure ─────────────────────────────────────────────────

func TestDelete_PartialFailure(t *testing.T) {
	dir := t.TempDir()

	good := filepath.Join(dir, "good.txt")
	if err := os.WriteFile(good, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	// dangerous path mixed in.
	ctx := context.Background()
	result, err := Delete(ctx, []string{"/usr", good}, false)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// /usr → error, good.txt → deleted.
	if len(result.Errors) != 1 {
		t.Errorf("Errors = %v; want 1", result.Errors)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != good {
		t.Errorf("Deleted = %v; want [%s]", result.Deleted, good)
	}
}

// ─── Delete — real directory removal ─────────────────────────────────────────

func TestDelete_RemovesDirectory(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "app_cache")
	if err := os.MkdirAll(filepath.Join(target, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "sub", "data.db"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	result, err := Delete(ctx, []string{target}, false)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if len(result.Deleted) != 1 {
		t.Errorf("Deleted = %v; want 1", result.Deleted)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Error("directory still exists after Delete")
	}
}

// ─── Delete — context cancellation ───────────────────────────────────────────

func TestDelete_ContextCancelled(t *testing.T) {
	dir := t.TempDir()

	// Create multiple files so there are multiple iterations to cancel through.
	var paths []string
	for i := range 5 {
		f := filepath.Join(dir, "f"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, f)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately before calling Delete.

	result, err := Delete(ctx, paths, false)
	// The function should return ctx.Err() and stop immediately.
	if err == nil {
		t.Error("Delete with cancelled context: expected error, got nil")
	}
	// Fewer than all 5 files should have been processed.
	totalProcessed := len(result.Deleted) + len(result.Skipped) + len(result.Errors)
	if totalProcessed > 0 {
		t.Logf("processed %d item(s) before cancel (acceptable)", totalProcessed)
	}
}
