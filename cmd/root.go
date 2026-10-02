package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/yourusername/cleanser/internal/app"
	"github.com/yourusername/cleanser/internal/deleter"
	"github.com/yourusername/cleanser/internal/logger"
	"github.com/yourusername/cleanser/internal/scanner"
	"github.com/yourusername/cleanser/internal/ui"
)

const version = "1.0.0"

var (
	appNames []string
	dryRun   bool
)

var rootCmd = &cobra.Command{
	Use:   "cleanser",
	Short: "Thoroughly uninstall macOS applications and all their related files",
	Long: `cleanser is a macOS CLI tool that removes applications and every trace they leave
behind — caches, preferences, logs, containers, and more.

Example:
  cleanser -a chrome
  cleanser -a chrome -a brave --dry-run`,
	Version:      version,
	RunE:         run,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringArrayVarP(&appNames, "app", "a", nil,
		"Application name(s) to uninstall (repeatable: -a chrome -a brave)")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"Simulate scanning and display results without deleting anything")

	if err := rootCmd.MarkFlagRequired("app"); err != nil {
		panic(err)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ui.Banner()

	if dryRun {
		ui.PrintWarning("Dry-run mode enabled — no files will be deleted.")
	}

	searcher := scanner.NewSearcher()

	var overallErr error
	for _, name := range appNames {
		if err := processApp(ctx, name, searcher); err != nil {
			ui.PrintError(fmt.Sprintf("Failed to process '%s': %v", name, err))
			overallErr = err
		}
		if ctx.Err() != nil {
			ui.PrintWarning("Operation cancelled by user.")
			break
		}
	}
	return overallErr
}

// processApp handles the full lifecycle for one application.
//
// Flow:
//  1. Discovery  — find .app, read Bundle ID
//  2. Process    — detect if running, offer force-quit
//  3. Scan       — collect related files (scored + filtered)
//  4. Display    — show list (dry-run) OR interactive checklist
//  5. Delete     — remove only user-selected entries
//  6. Log        — write audit trail
func processApp(ctx context.Context, name string, searcher scanner.Searcher) error {
	fmt.Println()

	// ── 1. Discovery ──────────────────────────────────────────────────────────
	ui.PrintInfo(fmt.Sprintf("Looking up '%s' in /Applications…", name))

	appInfo, err := app.FindApp(name)
	if err != nil {
		return fmt.Errorf("app discovery: %w", err)
	}
	ui.PrintAppHeader(appInfo.DisplayName, appInfo.Path, appInfo.SizeBytes)

	// ── 2. Process check ──────────────────────────────────────────────────────
	running, pids, err := app.IsRunning(appInfo.Name)
	if err != nil {
		ui.PrintWarning(fmt.Sprintf("Could not check if '%s' is running: %v", appInfo.DisplayName, err))
	}
	if running {
		msg := fmt.Sprintf("'%s' is currently running. Force quit it?", appInfo.DisplayName)
		if ui.ConfirmPrompt(msg) {
			if err := app.TerminateApp(pids); err != nil {
				ui.PrintWarning(fmt.Sprintf("Could not fully terminate '%s': %v", appInfo.DisplayName, err))
			} else {
				ui.PrintInfo(fmt.Sprintf("'%s' terminated.", appInfo.DisplayName))
			}
		} else {
			ui.PrintWarning("Proceeding without quitting the app. Some files may be locked.")
		}
	}

	// ── 3. Scan ───────────────────────────────────────────────────────────────
	ui.PrintInfo("Scanning Library directories for related files…")

	entries, err := searcher.Scan(ctx, appInfo)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("scan: %w", err)
	}

	// ── 4. Display ────────────────────────────────────────────────────────────
	if dryRun {
		// Dry-run: read-only list, no checklist prompt.
		ui.PrintDryRunList(entries)
		ui.PrintDryRunNotice()
		return nil
	}

	if len(entries) == 0 {
		ui.PrintInfo("No related files found. Nothing to delete.")
		return nil
	}

	// Interactive checklist — all unchecked by default.
	selected, err := ui.SelectFilesToDelete(entries)
	if err != nil {
		return fmt.Errorf("ui: %w", err)
	}

	if len(selected) == 0 {
		ui.PrintInfo("No files selected. Nothing was deleted.")
		return nil
	}

	// Summary of what the user chose.
	totalSize := ui.SumSize(selected)
	ui.PrintInfo(fmt.Sprintf(
		"%d file(s) selected (%s). Also removing app bundle: %s",
		len(selected), formatSize(totalSize), appInfo.Path,
	))

	// Final confirmation before irreversible action.
	msg := fmt.Sprintf("Permanently delete '%s' and %d selected file(s)?", appInfo.DisplayName, len(selected))
	if !ui.ConfirmPrompt(msg) {
		ui.PrintInfo("Deletion cancelled. No files were removed.")
		return nil
	}

	// ── 5. Delete ─────────────────────────────────────────────────────────────
	// Always delete the app bundle itself; then the user-selected library files.
	paths := []string{appInfo.Path}
	for _, e := range selected {
		paths = append(paths, e.Path)
	}

	ui.PrintInfo("Deleting files…")
	result, err := deleter.Delete(ctx, paths, false)
	if err != nil && ctx.Err() != nil {
		ui.PrintWarning("Deletion interrupted by user.")
	}

	ui.PrintDeleteResult(result.Deleted, result.Skipped, result.Errors)
	ui.PrintSuccess(appInfo.DisplayName, len(result.Deleted))

	// ── 6. Log ────────────────────────────────────────────────────────────────
	session := logger.NewSession(appInfo.DisplayName, false)
	session.Deleted = result.Deleted
	session.Skipped = result.Skipped

	if logErr := logger.LogSession(session); logErr != nil {
		ui.PrintWarning(fmt.Sprintf("Could not write audit log: %v", logErr))
	} else {
		ui.PrintInfo(fmt.Sprintf("Audit log updated: %s", logger.LogPath()))
	}

	return err
}

// formatSize is a local helper used before the ui package is available.
func formatSize(bytes int64) string {
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
