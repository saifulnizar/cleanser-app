# Cleanser

> A powerful CLI tool for macOS to thoroughly uninstall applications and remove all related files.

## Overview

**Cleanser** is a command-line interface (CLI) tool for macOS that completely removes applications and all their associated files (caches, preferences, logs, configurations, and more). Unlike the default macOS uninstall method (dragging to Trash), Cleanser ensures no traces are left behind.

## Features

- 🧹 **Complete Removal**: Removes app bundles and all related files from Library directories
- 🔍 **Smart Discovery**: Automatically finds apps and their Bundle Identifiers
- ⚡ **Fast Scanning**: Uses ripgrep for lightning-fast file discovery (with fallback to native tools)
- 📊 **Detailed Preview**: Shows exactly what will be deleted before confirmation
- 🔐 **Safe Authentication**: Uses native macOS authentication (Touch ID/Password)
- 🎯 **Dry-Run Mode**: Preview changes without actually deleting files
- 📝 **Audit Trail**: Logs all operations to `~/.cleanser_history.log`
- 🚀 **Multi-App Support**: Uninstall multiple applications in one command
- 💪 **Process Management**: Automatically handles running applications

## Installation

### Prerequisites

- macOS (darwin/arm64 or darwin/amd64)
- Go 1.20+ (for building from source)

### From Source

```bash
git clone https://github.com/saifulnizar/cleanser-app.git
cd cleanser-app
go build -o cleanser
sudo mv cleanser /usr/local/bin/
```

### From GitHub Releases

*(Coming soon - binaries will be available in GitHub Releases)*

## Usage

### Basic Usage

```bash
# Uninstall a single application
cleanser -a chrome

# Uninstall multiple applications
cleanser -a chrome -a safari -a firefox

# Preview what would be deleted (dry-run)
cleanser -a chrome --dry-run

# Show version
cleanser --version

# Show help
cleanser --help
```

### Examples

**Remove Google Chrome completely:**
```bash
cleanser -a chrome
```

**Preview Firefox removal without deleting:**
```bash
cleanser -a firefox --dry-run
```

**Remove multiple browsers at once:**
```bash
cleanser -a chrome -a safari -a firefox
```

## How It Works

1. **Discovery**: Finds the application in `/Applications` and extracts its Bundle Identifier
2. **Process Check**: Detects if the app is running and offers to quit it
3. **Scanning**: Searches Library directories for all related files using the app name and Bundle ID
4. **Preview**: Shows detailed information about what will be deleted
5. **Confirmation**: Asks for user confirmation before proceeding
6. **Authentication**: Requests macOS authentication (Touch ID/Password) for file deletion
7. **Deletion**: Removes all files using elevated privileges
8. **Logging**: Records the operation in the audit trail

## Directories Scanned

Cleanser searches the following directories for related files:

- `~/Library/Application Support/`
- `~/Library/Caches/`
- `~/Library/Preferences/`
- `~/Library/Logs/`
- `~/Library/Saved Application State/`
- `~/Library/Containers/`

## Safety

- ⚠️ **Always review** the list of files before confirming deletion
- 🛡️ **Use dry-run mode** first to preview changes
- 📋 **Check audit logs** at `~/.cleanser_history.log` for deletion history
- 🚫 **System apps** are protected and cannot be removed

## Development

### Project Structure

```
cleanser/
├── cmd/                    # Cobra CLI commands
│   └── root.go            # Root command implementation
├── internal/              # Internal packages
│   ├── app/              # App discovery & process management
│   ├── scanner/          # File scanning (ripgrep/fallback)
│   ├── deleter/          # File deletion with AppleScript
│   ├── logger/           # Audit trail logging
│   └── ui/               # User interface & prompts
├── main.go               # Entry point
├── go.mod                # Go module definition
└── README.md             # This file
```

### Building

```bash
# Build for current platform
go build -o cleanser

# Build for all macOS platforms
GOOS=darwin GOARCH=amd64 go build -o cleanser-amd64
GOOS=darwin GOARCH=arm64 go build -o cleanser-arm64
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/app/...
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

See [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) CLI framework
- Uses [ripgrep](https://github.com/BurntSushi/ripgrep) for fast file searching (optional)

## Roadmap

- [x] Basic CLI structure with Cobra
- [ ] App discovery and Bundle ID resolution
- [ ] Running process detection and termination
- [ ] Ripgrep-based scanning
- [ ] Fallback scanning (grep/find/mdfind)
- [ ] Interactive UI and confirmation
- [ ] AppleScript-based file deletion
- [ ] Dry-run mode
- [ ] Audit trail logging
- [ ] Multi-app support
- [ ] Comprehensive error handling
- [ ] Unit tests with >80% coverage
- [ ] GoReleaser setup for GitHub releases

---

**⚠️ Warning**: This tool permanently deletes files. Always use `--dry-run` first and review what will be deleted. The authors are not responsible for any data loss.
