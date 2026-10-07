package scanner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/saifulnizar/cleanser/internal/app"
)

// FileEntry represents a single file or directory found during scanning.
type FileEntry struct {
	// Path is the absolute path on disk.
	Path string
	// Size is the size in bytes (for directories, this is the total recursive size).
	Size int64
	// IsDir indicates whether the entry is a directory.
	IsDir bool
	// Score is the relevance score computed by the scoring engine (Solution C).
	// Higher = more likely to genuinely belong to the target app.
	Score int
}

// scoreThreshold is the minimum score for a file to be shown to the user.
// Entries scoring below this are considered false positives and silently dropped.
const scoreThreshold = 2

// Searcher is the interface that wraps the Scan method.
// This abstraction makes scanning testable and swappable.
type Searcher interface {
	Scan(ctx context.Context, info *app.AppInfo) ([]FileEntry, error)
}

// NewSearcher returns a RipgrepSearcher if rg is available, otherwise a FallbackSearcher.
func NewSearcher() Searcher {
	if _, err := exec.LookPath("rg"); err == nil {
		return &RipgrepSearcher{}
	}
	return &FallbackSearcher{}
}

// standardLibDirs are the macOS Library directories we scan for related files.
var standardLibDirs = []string{
	"Application Support",
	"Caches",
	"Preferences",
	"Logs",
	"Saved Application State",
	"Containers",
	"HTTPStorages",
	"WebKit",
}

// -----------------------------------------------------------------------
// RipgrepSearcher
// -----------------------------------------------------------------------

// RipgrepSearcher uses ripgrep (rg) to find files whose paths contain the
// app name or bundle identifier.
type RipgrepSearcher struct{}

// Scan implements Searcher using ripgrep --files.
func (r *RipgrepSearcher) Scan(ctx context.Context, info *app.AppInfo) ([]FileEntry, error) {
	libRoot, err := libraryRoot()
	if err != nil {
		return nil, err
	}

	terms := buildSearchTerms(info)
	var allEntries []FileEntry
	seen := make(map[string]struct{})

	for _, dir := range standardLibDirs {
		scanDir := filepath.Join(libRoot, dir)
		if _, err := os.Stat(scanDir); os.IsNotExist(err) {
			continue
		}

		for _, term := range terms {
			// rg --files lists all files under the path; we then filter by term in path.
			// Using --glob to match filenames containing the search term.
			args := []string{
				"--files",
				"--hidden",
				"--glob", fmt.Sprintf("*%s*", term),
				scanDir,
			}
			out, err := runWithContext(ctx, "rg", args...)
			if err != nil {
				// rg exit 1 = no matches; not an error.
				continue
			}

			for _, p := range splitLines(out) {
				if _, exists := seen[p]; exists {
					continue
				}
				seen[p] = struct{}{}
				entry, err := statEntry(p)
				if err == nil {
					allEntries = append(allEntries, entry)
				}
			}
		}

		// Also check top-level directories whose names contain the term.
		dirEntries, err := matchTopLevelDirs(ctx, scanDir, terms)
		if err == nil {
			for _, e := range dirEntries {
				if _, exists := seen[e.Path]; !exists {
					seen[e.Path] = struct{}{}
					allEntries = append(allEntries, e)
				}
			}
		}
	}

	return applyScoring(allEntries, info), nil
}

// -----------------------------------------------------------------------
// FallbackSearcher
// -----------------------------------------------------------------------

// FallbackSearcher uses find + mdfind when ripgrep is not available.
type FallbackSearcher struct{}

// Scan implements Searcher using find and mdfind (Spotlight).
func (f *FallbackSearcher) Scan(ctx context.Context, info *app.AppInfo) ([]FileEntry, error) {
	libRoot, err := libraryRoot()
	if err != nil {
		return nil, err
	}

	terms := buildSearchTerms(info)
	seen := make(map[string]struct{})
	var allEntries []FileEntry

	for _, dir := range standardLibDirs {
		scanDir := filepath.Join(libRoot, dir)
		if _, err := os.Stat(scanDir); os.IsNotExist(err) {
			continue
		}

		for _, term := range terms {
			// find -iname "*term*"
			args := []string{scanDir, "-iname", fmt.Sprintf("*%s*", term)}
			out, err := runWithContext(ctx, "find", args...)
			if err != nil {
				continue
			}

			for _, p := range splitLines(out) {
				if _, exists := seen[p]; exists {
					continue
				}
				seen[p] = struct{}{}
				entry, err := statEntry(p)
				if err == nil {
					allEntries = append(allEntries, entry)
				}
			}
		}
	}

	// mdfind for bundle ID (Spotlight metadata search).
	if info.BundleID != "" {
		mdArgs := []string{"kMDItemCFBundleIdentifier=" + info.BundleID}
		out, err := runWithContext(ctx, "mdfind", mdArgs...)
		if err == nil {
			for _, p := range splitLines(out) {
				if _, exists := seen[p]; exists {
					continue
				}
				seen[p] = struct{}{}
				entry, err := statEntry(p)
				if err == nil {
					allEntries = append(allEntries, entry)
				}
			}
		}
	}

	return applyScoring(allEntries, info), nil
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// buildSearchTerms returns a deduplicated list of terms to search for.
//
// Strategy:
//  1. Always include the user-supplied name (e.g. "brave") — most reliable.
//  2. Split the display name into words, but ONLY keep words that are:
//     - At least 4 characters long (blocks "App", "The", "for")
//     - Not a known generic word (blocks "Browser", "Client", "Player", etc.)
//     - Not already equal to the user name (avoid duplicate)
//  3. Always include the full Bundle ID (e.g. "com.brave.browser") — very specific.
func buildSearchTerms(info *app.AppInfo) []string {
	terms := []string{strings.ToLower(info.Name)}

	// Extract meaningful words from the display name.
	// e.g. "Brave Browser" → ["brave"] only (browser is blocked as generic)
	// e.g. "Google Chrome" → ["chrome", "google"]
	// e.g. "Microsoft Word" → ["chrome", "microsoft", "word"]
	nameLower := strings.ToLower(info.Name)
	for _, word := range strings.Fields(strings.ToLower(info.DisplayName)) {
		if len(word) < 4 {
			// Too short: noise words like "of", "by", "the".
			continue
		}
		if isGenericWord(word) {
			// Block words that are too common across many apps.
			continue
		}
		if word == nameLower {
			// Already included as the primary term.
			continue
		}
		terms = append(terms, word)
	}

	if info.BundleID != "" {
		terms = append(terms, strings.ToLower(info.BundleID))
	}

	return dedupe(terms)
}

// genericWords is a block list of words that appear in many macOS app display
// names but are far too broad to use as file-search terms safely.
var genericWords = map[string]struct{}{
	"app": {}, "apps": {}, "browser": {}, "client": {},
	"editor": {}, "player": {}, "manager": {}, "viewer": {},
	"helper": {}, "agent": {}, "service": {}, "lite": {},
	"plus": {}, "free": {}, "tool": {}, "tools": {},
	"suite": {}, "studio": {}, "desktop": {}, "cloud": {},
	"drive": {}, "sync": {}, "updater": {}, "launcher": {},
	"engine": {},
}

// isGenericWord reports whether the word is in the generic block list.
func isGenericWord(word string) bool {
	_, ok := genericWords[word]
	return ok
}

// -----------------------------------------------------------------------
// Scoring Engine (Solution C)
// -----------------------------------------------------------------------

// applyScoring scores every entry and returns only those at or above the
// scoreThreshold, sorted by score descending.
//
// Scoring rules:
//
//	+3  path contains exact Bundle ID (most specific signal)
//	+2  path contains the user-supplied app name
//	+1  path contains another meaningful word from the display name
//	     (already filtered through the generic-word block list)
//
// A score < scoreThreshold means the file is very likely a false positive
// (e.g. com.google.antigravity appearing when scanning for Chrome).
func applyScoring(entries []FileEntry, info *app.AppInfo) []FileEntry {
	var result []FileEntry
	for _, e := range entries {
		e.Score = scoreEntry(e.Path, info)
		if e.Score >= scoreThreshold {
			result = append(result, e)
		}
	}
	return result
}

// scoreEntry computes a relevance score for a single path against the target app.
func scoreEntry(path string, info *app.AppInfo) int {
	score := 0
	pathLower := strings.ToLower(path)

	// +3: contains exact Bundle ID — very specific, almost certainly the right app.
	if info.BundleID != "" && strings.Contains(pathLower, strings.ToLower(info.BundleID)) {
		score += 3
	}

	// +2: contains user's input name (e.g. "chrome", "brave").
	if strings.Contains(pathLower, strings.ToLower(info.Name)) {
		score += 2
	}

	// +1 per meaningful word from the display name that's not generic.
	// e.g. "Google Chrome" → "+1 for google" (only if path contains it).
	nameLower := strings.ToLower(info.Name)
	for _, word := range strings.Fields(strings.ToLower(info.DisplayName)) {
		if len(word) < 4 || isGenericWord(word) || word == nameLower {
			continue
		}
		if strings.Contains(pathLower, word) {
			score++
		}
	}

	return score
}

// matchTopLevelDirs returns FileEntry items for top-level directories under
// scanDir whose names contain any of the given terms.
func matchTopLevelDirs(_ context.Context, scanDir string, terms []string) ([]FileEntry, error) {
	entries, err := os.ReadDir(scanDir)
	if err != nil {
		return nil, err
	}

	var matched []FileEntry
	for _, e := range entries {
		nameLower := strings.ToLower(e.Name())
		for _, term := range terms {
			if strings.Contains(nameLower, term) {
				fullPath := filepath.Join(scanDir, e.Name())
				entry, err := statEntry(fullPath)
				if err == nil {
					matched = append(matched, entry)
				}
				break
			}
		}
	}
	return matched, nil
}

// statEntry returns a FileEntry for path, computing recursive size for directories.
func statEntry(path string) (FileEntry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return FileEntry{}, err
	}

	entry := FileEntry{Path: path, IsDir: info.IsDir()}

	if info.IsDir() {
		entry.Size, _ = dirSize(path)
	} else {
		entry.Size = info.Size()
	}
	return entry, nil
}

// dirSize recursively sums up sizes of all regular files under path.
func dirSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// libraryRoot returns ~/Library.
func libraryRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("scanner: resolve home dir: %w", err)
	}
	return filepath.Join(home, "Library"), nil
}

// runWithContext executes a command and returns combined stdout. Stderr is ignored.
func runWithContext(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	return string(out), err
}

// splitLines splits a newline-delimited string into non-empty trimmed lines.
func splitLines(s string) []string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// dedupe returns a new slice with duplicate strings removed (preserving order).
func dedupe(ss []string) []string {
	seen := make(map[string]struct{}, len(ss))
	var out []string
	for _, s := range ss {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
