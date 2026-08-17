# Contributing to Orpheus

First off, thank you for considering contributing to Orpheus! 🎉

## Development Setup

### Prerequisites
- **Go 1.26+**
- **Arch Linux** (or an Arch-based distribution)
- Optional: `yay` or `paru`, `flatpak`

### Building and Running Locally

```bash
# Clone the repository
git clone https://github.com/Hendrixx-RE/pacseer.git
cd pacseer

# Build binary
make build

# Run unit tests
make test

# Launch Orpheus
./pacseer
```

## Project Structure

- `main.go`: Application entry point and environment loader.
- `internal/pm/`: Package manager abstraction layer (`Manager` interface, Pacman, Flatpak, fuzzy ranker).
- `internal/ai/`: Groq AI integration with singleflight deduplication and rate-limit circuit breakers.
- `internal/cache/`: Thread-safe JSON analysis cache.
- `internal/tui/`: Bubble Tea TUI architecture (`model.go`, `update.go`, `view.go`, `styles.go`).

## Code Guidelines

- Ensure code passes `go test ./...` and `go vet ./...`.
- Format code using `gofmt` or `goimports`.
- When adding a new package manager, implement the `pm.Manager` interface in `internal/pm/`.
- Maintain test coverage for any new utilities, parsers, or deduplication logic.

## Submitting Pull Requests

1. Fork the repo and create your branch from `main`.
2. Ensure all tests pass (`make test`).
3. Commit your changes with clear, descriptive commit messages.
4. Open a Pull Request referencing any related issues.
