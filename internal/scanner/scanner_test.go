package scanner

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/yourusername/cleanser/internal/app"
)

// ─── buildSearchTerms ─────────────────────────────────────────────────────────

func TestBuildSearchTerms_NameOnly(t *testing.T) {
	info := &app.AppInfo{Name: "chrome", DisplayName: "Google Chrome"}
	terms := buildSearchTerms(info)

	mustContain(t, terms, "chrome")
	mustContain(t, terms, "google")
	// "chrome" from display name deduped with name input.
	if count(terms, "chrome") != 1 {
		t.Errorf("'chrome' appears %d times; want 1 (deduped)", count(terms, "chrome"))
	}
}

func TestBuildSearchTerms_BrowserWordBlocked(t *testing.T) {
	// "Browser" is a generic word and must NOT appear as a search term.
	info := &app.AppInfo{Name: "brave", DisplayName: "Brave Browser"}
	terms := buildSearchTerms(info)

	mustContain(t, terms, "brave")
	for _, term := range terms {
		if term == "browser" {
			t.Error("'browser' must be blocked as a generic word; got it in terms")
		}
	}
}

func TestBuildSearchTerms_WithBundleID(t *testing.T) {
	info := &app.AppInfo{
		Name:        "chrome",
		DisplayName: "Google Chrome",
		BundleID:    "com.google.Chrome",
	}
	terms := buildSearchTerms(info)

	mustContain(t, terms, "chrome")
	mustContain(t, terms, "google")
	mustContain(t, terms, "com.google.chrome")
}

func TestBuildSearchTerms_SingleWordDisplayName(t *testing.T) {
	info := &app.AppInfo{Name: "vlc", DisplayName: "VLC"}
	terms := buildSearchTerms(info)

	mustContain(t, terms, "vlc")
	// "vlc" in display name equals name — deduped to exactly one entry.
	if count(terms, "vlc") != 1 {
		t.Errorf("'vlc' appears %d times; want 1", count(terms, "vlc"))
	}
}

func TestBuildSearchTerms_EmptyBundleID(t *testing.T) {
	info := &app.AppInfo{Name: "test", DisplayName: "Test App", BundleID: ""}
	terms := buildSearchTerms(info)
	for _, term := range terms {
		if term == "" {
			t.Error("buildSearchTerms returned an empty-string term")
		}
	}
}

// ─── dedupe ──────────────────────────────────────────────────────────────────

func TestDedupe_NoDuplicates(t *testing.T) {
	input := []string{"a", "b", "c"}
	got := dedupe(input)
	if len(got) != 3 {
		t.Errorf("dedupe len = %d; want 3", len(got))
	}
}

func TestDedupe_AllDuplicates(t *testing.T) {
	input := []string{"x", "x", "x"}
	got := dedupe(input)
	if len(got) != 1 || got[0] != "x" {
		t.Errorf("dedupe = %v; want [x]", got)
	}
}

func TestDedupe_PreservesOrder(t *testing.T) {
	input := []string{"c", "a", "b", "a", "c"}
	got := dedupe(input)
	want := []string{"c", "a", "b"}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("dedupe[%d] = %q; want %q", i, got[i], v)
		}
	}
}

func TestDedupe_Empty(t *testing.T) {
	got := dedupe(nil)
	if len(got) != 0 {
		t.Errorf("dedupe(nil) = %v; want []", got)
	}
}

// ─── splitLines ──────────────────────────────────────────────────────────────

func TestSplitLines_Empty(t *testing.T) {
	got := splitLines("")
	if len(got) != 0 {
		t.Errorf("splitLines(\"\") = %v; want []", got)
	}
}

func TestSplitLines_SingleLine(t *testing.T) {
	got := splitLines("/path/to/file\n")
	if len(got) != 1 || got[0] != "/path/to/file" {
		t.Errorf("splitLines = %v; want [/path/to/file]", got)
	}
}

func TestSplitLines_MultipleLines(t *testing.T) {
	input := "/a\n/b\n/c\n"
	got := splitLines(input)
	if len(got) != 3 {
		t.Errorf("splitLines len = %d; want 3", len(got))
	}
}

func TestSplitLines_SkipsBlankLines(t *testing.T) {
	input := "/a\n\n  \n/b\n"
	got := splitLines(input)
	if len(got) != 2 {
		t.Errorf("splitLines = %v; want 2 entries", got)
	}
}

// ─── statEntry ───────────────────────────────────────────────────────────────

func TestStatEntry_File(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry, err := statEntry(f)
	if err != nil {
		t.Fatalf("statEntry: %v", err)
	}

	if entry.IsDir {
		t.Error("statEntry: IsDir = true for a file")
	}
	if entry.Size != 5 {
		t.Errorf("statEntry: Size = %d; want 5", entry.Size)
	}
	if entry.Path != f {
		t.Errorf("statEntry: Path = %q; want %q", entry.Path, f)
	}
}

func TestStatEntry_Directory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "f.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry, err := statEntry(sub)
	if err != nil {
		t.Fatalf("statEntry: %v", err)
	}

	if !entry.IsDir {
		t.Error("statEntry: IsDir = false for a directory")
	}
	if entry.Size != 3 {
		t.Errorf("statEntry: Size = %d; want 3", entry.Size)
	}
}

func TestStatEntry_NonExistent(t *testing.T) {
	_, err := statEntry("/tmp/__cleanser_no_such_entry_xyz__")
	if err == nil {
		t.Error("statEntry: expected error for non-existent path, got nil")
	}
}

// ─── matchTopLevelDirs ────────────────────────────────────────────────────────

func TestMatchTopLevelDirs_Matches(t *testing.T) {
	dir := t.TempDir()

	// Create directories: one that matches "chrome", one that doesn't.
	chromeDir := filepath.Join(dir, "Google Chrome")
	otherDir := filepath.Join(dir, "Firefox")
	for _, d := range []string{chromeDir, otherDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := matchTopLevelDirs(context.Background(), dir, []string{"chrome"})
	if err != nil {
		t.Fatalf("matchTopLevelDirs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("matchTopLevelDirs: got %d entries; want 1", len(got))
	}
	if got[0].Path != chromeDir {
		t.Errorf("matchTopLevelDirs: path = %q; want %q", got[0].Path, chromeDir)
	}
}

func TestMatchTopLevelDirs_NoMatches(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Slack"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := matchTopLevelDirs(context.Background(), dir, []string{"chrome"})
	if err != nil {
		t.Fatalf("matchTopLevelDirs: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("matchTopLevelDirs: got %v; want empty", got)
	}
}

func TestMatchTopLevelDirs_MultipleTerms(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"Google Chrome", "com.google.Chrome", "Mozilla Firefox"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := matchTopLevelDirs(context.Background(), dir, []string{"chrome", "google"})
	if err != nil {
		t.Fatalf("matchTopLevelDirs: %v", err)
	}

	paths := make([]string, len(got))
	for i, e := range got {
		paths[i] = filepath.Base(e.Path)
	}
	sort.Strings(paths)

	// "Google Chrome" matches "chrome" AND "google" but must appear only once.
	// "com.google.Chrome" matches "chrome" and "google".
	// "Mozilla Firefox" matches nothing.
	// Total unique matches: 2.
	if len(paths) != 2 {
		t.Errorf("matchTopLevelDirs: got %d entries %v; want 2", len(paths), paths)
	}
}

func TestMatchTopLevelDirs_NonExistentDir(t *testing.T) {
	_, err := matchTopLevelDirs(context.Background(), "/tmp/__cleanser_no_dir__", []string{"app"})
	if err == nil {
		t.Error("matchTopLevelDirs: expected error for non-existent dir, got nil")
	}
}

// ─── FallbackSearcher — integration ──────────────────────────────────────────

// TestFallbackSearcher_ScanFindsFiles creates a fake Library-style directory
// structure and verifies that FallbackSearcher locates files matching the app name.
func TestFallbackSearcher_ScanFindsFiles(t *testing.T) {
	// Build a fake ~/Library with a Caches directory containing app-named files.
	fakeHome := t.TempDir()
	cacheDir := filepath.Join(fakeHome, "Library", "Caches")
	appCacheDir := filepath.Join(cacheDir, "com.testapp.cleanser")
	if err := os.MkdirAll(appCacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appCacheDir, "data.bin"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override HOME so libraryRoot() points at fakeHome.
	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", fakeHome); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("HOME", origHome)

	info := &app.AppInfo{
		Name:        "testapp",
		DisplayName: "TestApp Cleanser",
		BundleID:    "com.testapp.cleanser",
	}

	fs := &FallbackSearcher{}
	entries, err := fs.Scan(context.Background(), info)
	if err != nil {
		t.Fatalf("FallbackSearcher.Scan: %v", err)
	}

	// At minimum, the appCacheDir or the file inside it should be found.
	if len(entries) == 0 {
		t.Error("FallbackSearcher.Scan: no entries found; expected at least 1")
	}

	// Verify all returned paths actually exist.
	for _, e := range entries {
		if _, err := os.Lstat(e.Path); err != nil {
			t.Errorf("entry path %q does not exist: %v", e.Path, err)
		}
	}
}

func TestFallbackSearcher_ScanNothingFound(t *testing.T) {
	fakeHome := t.TempDir()
	// Create standard Library dirs but no matching files.
	if err := os.MkdirAll(filepath.Join(fakeHome, "Library", "Caches"), 0o755); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", fakeHome); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("HOME", origHome)

	info := &app.AppInfo{
		Name:        "__no_match_xyz__",
		DisplayName: "__no_match_xyz__",
	}

	fs := &FallbackSearcher{}
	entries, err := fs.Scan(context.Background(), info)
	if err != nil {
		t.Fatalf("FallbackSearcher.Scan: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("FallbackSearcher.Scan: got %d entries; want 0", len(entries))
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func mustContain(t *testing.T, ss []string, want string) {
	t.Helper()
	for _, s := range ss {
		if s == want {
			return
		}
	}
	t.Errorf("slice %v does not contain %q", ss, want)
}

func count(ss []string, target string) int {
	n := 0
	for _, s := range ss {
		if s == target {
			n++
		}
	}
	return n
}
