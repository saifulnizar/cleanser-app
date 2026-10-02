package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/yourusername/cleanser/internal/scanner"
)

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorGray   = "\033[90m"
)

// Banner prints the cleanser header banner.
func Banner() {
	fmt.Printf("\n%s%s╔══════════════════════════════════╗%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s║        🧹  C L E A N S E R       ║%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s╚══════════════════════════════════╝%s\n\n", colorBold, colorCyan, colorReset)
}

// PrintAppHeader prints the summary block for a discovered application.
func PrintAppHeader(appName, appPath string, appSizeBytes int64) {
	fmt.Printf("%s%s▸ App Name  :%s %s\n", colorBold, colorGreen, colorReset, appName)
	fmt.Printf("%s%s▸ App Path  :%s %s\n", colorBold, colorGreen, colorReset, appPath)
	fmt.Printf("%s%s▸ App Size  :%s %s\n", colorBold, colorGreen, colorReset, formatSize(appSizeBytes))
}

// SelectFilesToDelete shows an interactive checklist of all scanned files.
// All items start UNCHECKED — user actively selects which ones to delete.
// Returns the subset of entries the user chose, or nil if none selected / cancelled.
func SelectFilesToDelete(entries []scanner.FileEntry) ([]scanner.FileEntry, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	fmt.Printf("\n%s%s▸ App Related Files found (%d items):%s\n", colorBold, colorYellow, len(entries), colorReset)

	// Build option labels (unique per entry because path is unique).
	options := make([]string, len(entries))
	for i, e := range entries {
		icon := "📄"
		if e.IsDir {
			icon = "📁"
		}
		// Pad score indicator for alignment.
		var scoreTag string
		switch {
		case e.Score >= 5:
			scoreTag = fmt.Sprintf("%s[●●● high]%s", colorGreen, colorReset)
		case e.Score >= 3:
			scoreTag = fmt.Sprintf("%s[●●○ med] %s", colorYellow, colorReset)
		default:
			scoreTag = fmt.Sprintf("%s[●○○ low] %s", colorGray, colorReset)
		}

		options[i] = fmt.Sprintf("%s  %s  %s  %s",
			scoreTag,
			icon,
			e.Path,
			fmt.Sprintf("%s(%s)%s", colorGray, formatSize(e.Size), colorReset),
		)
	}

	fmt.Printf("%s  Space=toggle  ↑↓=navigate  a=select all  Enter=confirm%s\n\n",
		colorGray, colorReset)

	// Survey MultiSelect: all unchecked by default (no Default field set).
	var selectedLabels []string
	prompt := &survey.MultiSelect{
		Message:  "Select files to DELETE:",
		Options:  options,
		PageSize: 20,
	}

	err := survey.AskOne(prompt, &selectedLabels, survey.WithIcons(func(icons *survey.IconSet) {
		icons.MarkedOption.Text = "✓"
		icons.UnmarkedOption.Text = "○"
	}))
	if err != nil {
		// User hit Ctrl+C or Esc.
		if err == terminal.InterruptErr {
			return nil, nil
		}
		return nil, fmt.Errorf("ui: checklist: %w", err)
	}

	if len(selectedLabels) == 0 {
		return nil, nil
	}

	// Map selected labels back to FileEntry (match by index via options slice).
	selectedSet := make(map[string]struct{}, len(selectedLabels))
	for _, s := range selectedLabels {
		selectedSet[s] = struct{}{}
	}

	var result []scanner.FileEntry
	for i, e := range entries {
		if _, ok := selectedSet[options[i]]; ok {
			result = append(result, e)
		}
	}
	return result, nil
}

// PrintDryRunNotice prints a notice when --dry-run mode is active.
func PrintDryRunNotice() {
	fmt.Printf("\n%s%s[DRY-RUN] No files were deleted. This was a simulation only.%s\n\n",
		colorBold, colorYellow, colorReset)
}

// PrintDryRunList prints the scan results in dry-run mode (read-only, no checklist).
func PrintDryRunList(entries []scanner.FileEntry) {
	if len(entries) == 0 {
		fmt.Printf("\n%s  (No related files found in Library directories.)%s\n", colorGray, colorReset)
		return
	}

	fmt.Printf("\n%s%s▸ App Related Files (%d items):%s\n", colorBold, colorYellow, len(entries), colorReset)

	var totalSize int64
	for _, e := range entries {
		icon := "  📄"
		if e.IsDir {
			icon = "  📁"
		}
		fmt.Printf("%s %s%s%s %s(%s)%s\n",
			icon,
			colorCyan, e.Path, colorReset,
			colorGray, formatSize(e.Size), colorReset,
		)
		totalSize += e.Size
	}

	fmt.Printf("\n%s%s  Total related files size: %s%s\n", colorBold, colorYellow, formatSize(totalSize), colorReset)
}

// ConfirmPrompt asks the user a Y/N question.
func ConfirmPrompt(msg string) bool {
	var confirmed bool
	prompt := &survey.Confirm{
		Message: msg,
		Default: false,
	}
	if err := survey.AskOne(prompt, &confirmed); err != nil {
		return false
	}
	return confirmed
}

// PrintSuccess prints a success message after deletion.
func PrintSuccess(appName string, deletedCount int) {
	fmt.Printf("\n%s%s✅  Successfully cleaned '%s'. Removed %d item(s).%s\n\n",
		colorBold, colorGreen, appName, deletedCount, colorReset)
}

// PrintError prints a user-facing error message.
func PrintError(msg string) {
	fmt.Fprintf(os.Stderr, "%s%s✖  %s%s\n", colorBold, colorRed, msg, colorReset)
}

// PrintWarning prints a warning message.
func PrintWarning(msg string) {
	fmt.Printf("%s%s⚠  %s%s\n", colorBold, colorYellow, msg, colorReset)
}

// PrintInfo prints an informational message.
func PrintInfo(msg string) {
	fmt.Printf("%s%s➜  %s%s\n", colorBold, colorCyan, msg, colorReset)
}

// PrintDeleteResult summarizes the deletion outcome.
func PrintDeleteResult(deleted, skipped []string, errs []error) {
	if len(deleted) > 0 {
		fmt.Printf("\n%s%sDeleted (%d):%s\n", colorBold, colorGreen, len(deleted), colorReset)
		for _, p := range deleted {
			fmt.Printf("  %s✔%s %s\n", colorGreen, colorReset, p)
		}
	}

	if len(skipped) > 0 {
		fmt.Printf("\n%s%sSkipped / already removed (%d):%s\n", colorBold, colorGray, len(skipped), colorReset)
		for _, p := range skipped {
			fmt.Printf("  %s–%s %s\n", colorGray, colorReset, p)
		}
	}

	if len(errs) > 0 {
		fmt.Printf("\n%s%sErrors (%d):%s\n", colorBold, colorRed, len(errs), colorReset)
		for _, e := range errs {
			fmt.Printf("  %s✖%s %v\n", colorRed, colorReset, e)
		}
	}
}

// formatSize formats a byte count into a human-readable string.
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

// SumSize returns the total size in bytes of all given entries.
func SumSize(entries []scanner.FileEntry) int64 {
	var total int64
	for _, e := range entries {
		total += e.Size
	}
	return total
}

// bufio-based raw ConfirmPrompt is kept for non-TTY contexts (tests).
func confirmRaw(msg string) bool {
	fmt.Printf("\n%s%s%s%s [y/N] ", colorBold, colorYellow, msg, colorReset)
	var answer string
	fmt.Scanln(&answer)
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}
