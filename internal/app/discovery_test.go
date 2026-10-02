package app

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── FindApp ────────────────────────────────────────────────────────────────

func TestFindApp_NotFound(t *testing.T) {
	_, err := FindApp("__no_such_app_xyz_cleanser__")
	if err == nil {
		t.Fatal("expected error for non-existent app, got nil")
	}
}

func TestFindApp_EmptyName(t *testing.T) {
	// An empty search term technically matches everything whose name contains "".
	// We test that no panic occurs regardless of the result.
	_, _ = FindApp("")
}

// ─── readBundleID ────────────────────────────────────────────────────────────

func TestReadBundleID_Valid(t *testing.T) {
	// Create a minimal .app structure with a valid XML plist.
	dir := t.TempDir()
	appPath := filepath.Join(dir, "Test.app")
	contentsDir := filepath.Join(appPath, "Contents")
	if err := os.MkdirAll(contentsDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleIdentifier</key>
  <string>com.example.testapp</string>
  <key>CFBundleName</key>
  <string>TestApp</string>
</dict>
</plist>`

	plistPath := filepath.Join(contentsDir, "Info.plist")
	if err := os.WriteFile(plistPath, []byte(plistContent), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := readBundleID(appPath)
	if err != nil {
		t.Fatalf("readBundleID: unexpected error: %v", err)
	}
	if got != "com.example.testapp" {
		t.Errorf("readBundleID = %q; want %q", got, "com.example.testapp")
	}
}

func TestReadBundleID_MissingPlist(t *testing.T) {
	dir := t.TempDir()
	appPath := filepath.Join(dir, "NoPlist.app")
	if err := os.MkdirAll(filepath.Join(appPath, "Contents"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := readBundleID(appPath)
	if err == nil {
		t.Fatal("expected error for missing Info.plist, got nil")
	}
}

func TestReadBundleID_MalformedPlist(t *testing.T) {
	dir := t.TempDir()
	appPath := filepath.Join(dir, "BadPlist.app")
	contentsDir := filepath.Join(appPath, "Contents")
	if err := os.MkdirAll(contentsDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(contentsDir, "Info.plist"),
		[]byte("this is not valid plist content!!!"),
		0o644,
	); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := readBundleID(appPath)
	if err == nil {
		t.Fatal("expected error for malformed plist, got nil")
	}
}

// ─── dirSize ─────────────────────────────────────────────────────────────────

func TestDirSize_Empty(t *testing.T) {
	dir := t.TempDir()
	size, err := dirSize(dir)
	if err != nil {
		t.Fatalf("dirSize: %v", err)
	}
	if size != 0 {
		t.Errorf("dirSize(empty dir) = %d; want 0", size)
	}
}

func TestDirSize_WithFiles(t *testing.T) {
	dir := t.TempDir()

	// Write two files with known sizes.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world!!"), 0o644); err != nil {
		t.Fatal(err)
	}

	size, err := dirSize(dir)
	if err != nil {
		t.Fatalf("dirSize: %v", err)
	}
	// "hello" = 5 bytes, "world!!" = 7 bytes → 12 total.
	if size != 12 {
		t.Errorf("dirSize = %d; want 12", size)
	}
}

func TestDirSize_Nested(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "c.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}

	size, err := dirSize(dir)
	if err != nil {
		t.Fatalf("dirSize: %v", err)
	}
	if size != 3 {
		t.Errorf("dirSize(nested) = %d; want 3", size)
	}
}
