package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"
)

// AppInfo holds all metadata about a discovered application.
type AppInfo struct {
	// Name is the user-supplied search term (e.g. "chrome").
	Name string
	// DisplayName is the .app folder name without extension (e.g. "Google Chrome").
	DisplayName string
	// Path is the absolute path to the .app bundle (e.g. /Applications/Google Chrome.app).
	Path string
	// BundleID is the CFBundleIdentifier from Info.plist (e.g. com.google.Chrome).
	BundleID string
	// SizeBytes is the total size of the .app bundle in bytes.
	SizeBytes int64
}

// infoPlist is the subset of Info.plist we care about.
type infoPlist struct {
	BundleIdentifier string `plist:"CFBundleIdentifier"`
	BundleName       string `plist:"CFBundleName"`
}

// FindApp searches /Applications for an application matching name (case-insensitive).
// It returns an AppInfo populated with Bundle ID and size, or an error if not found.
func FindApp(name string) (*AppInfo, error) {
	entries, err := os.ReadDir("/Applications")
	if err != nil {
		return nil, fmt.Errorf("app: read /Applications: %w", err)
	}

	nameLower := strings.ToLower(name)

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasSuffix(entry.Name(), ".app") {
			continue
		}

		displayName := strings.TrimSuffix(entry.Name(), ".app")
		if !strings.Contains(strings.ToLower(displayName), nameLower) {
			continue
		}

		appPath := filepath.Join("/Applications", entry.Name())

		bundleID, err := readBundleID(appPath)
		if err != nil {
			// Non-fatal: proceed without Bundle ID (scan will fall back to name-only).
			bundleID = ""
		}

		size, err := dirSize(appPath)
		if err != nil {
			size = 0
		}

		return &AppInfo{
			Name:        name,
			DisplayName: displayName,
			Path:        appPath,
			BundleID:    bundleID,
			SizeBytes:   size,
		}, nil
	}

	return nil, fmt.Errorf("app: '%s' not found in /Applications", name)
}

// readBundleID parses the Info.plist inside an .app bundle and returns CFBundleIdentifier.
func readBundleID(appPath string) (string, error) {
	plistPath := filepath.Join(appPath, "Contents", "Info.plist")

	data, err := os.ReadFile(plistPath)
	if err != nil {
		return "", fmt.Errorf("app: read Info.plist at %s: %w", plistPath, err)
	}

	var info infoPlist
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return "", fmt.Errorf("app: parse Info.plist at %s: %w", plistPath, err)
	}

	return info.BundleIdentifier, nil
}

// dirSize returns the total size in bytes of all regular files under a directory.
// It silently skips files it cannot stat (e.g. broken symlinks).
func dirSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip unreadable entries rather than aborting the walk.
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
