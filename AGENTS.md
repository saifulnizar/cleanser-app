# Cleanser — Agent Context Guide

Hello fellow Agent! If you are reading this, you are working on the `cleanser` repository. This file provides the essential context, architectural decisions, and constraints of the project so you can contribute safely and effectively without reading every file.

## 🎯 Project Overview
`cleanser` is a macOS CLI tool written in Go that thoroughly uninstalls applications and their scattered related files (caches, preferences, containers, application support, etc.).

## 🏗 Architecture & Packages
The project follows a standard Go project layout:

- **`cmd/`**: CLI entry points using `spf13/cobra`.
  - Flow: Discovery ➔ Process Check ➔ Scan ➔ Interactive UI ➔ Delete ➔ Log.
- **`internal/app/`**: Handles macOS `.app` bundle discovery, parses `Info.plist` for the Bundle ID, and manages process termination (SIGTERM -> SIGKILL).
- **`internal/scanner/`**: Finds related files in `~/Library/*`. 
  - Prefers `ripgrep` (`rg`) for speed, falls back to `find` and `mdfind` (Spotlight).
  - **Crucial Concept**: Uses a **Scoring Engine** to prevent false positives.
- **`internal/ui/`**: Uses `AlecAivazis/survey/v2` for an interactive terminal checklist.
- **`internal/deleter/`**: Performs actual file removal. Contains hardcoded safety guards to prevent deleting critical paths (`/`, `/Applications`, `/Users`, etc.). Handles root elevation via AppleScript if permission is denied.
- **`internal/logger/`**: Maintains an append-only, idempotent audit log at `~/.cleanser_history.log` using UUID session tracking.

## ⚠️ Key Constraints & Design Patterns (MUST READ)

1. **The Scoring Engine (`internal/scanner/scanner.go`)**
   - We do not use naive string matching to find files, as it causes massive false positives (e.g., searching "browser" matches Safari files).
   - `applyScoring` assigns points: `+3` for Bundle ID exact match, `+2` for user input name match, `+1` for other non-generic display name words.
   - Files scoring below `scoreThreshold` (2) are silently dropped.
   - If you modify how search terms are generated, you MUST respect this scoring logic and update `genericWords` if necessary.

2. **Safety & Destructive Actions (`internal/ui/` & `internal/deleter/`)**
   - **Opt-In UI**: The interactive checklist (`SelectFilesToDelete`) MUST default all items to UNCHECKED. The user must actively select what to delete.
   - **Path Guards**: `deleter.guardPath` explicitly blocks deletion of top-level system directories. Never bypass this.

3. **Idempotency**
   - Deletion must not error if a file is already gone.
   - Process termination must not error if the process died before the signal reached it (`ESRCH` is ignored).
   - Logging must safely append without corrupting previous entries.

4. **Testing**
   - The project maintains high test coverage (~75%+). 
   - When adding features, ensure you write unit tests. The `deleter` and `scanner` packages are thoroughly tested with mock file systems or edge case inputs.

## 🛠 Commands
- Build: `go build -o cleanser .`
- Run Tests: `go test ./... -count=1`
- Run App (Safe Mode): `go run . -a <app_name> --dry-run`
