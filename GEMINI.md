# Orpheus — Project Reference for AI Agents

> **Orpheus** is a terminal-based (TUI) package management dashboard for Arch Linux.
> It lets users browse, inspect, AI-analyze, search, install, update, and batch-uninstall packages from multiple
> package managers (Pacman/AUR, Flatpak) in a single unified interface.

---

## Quick Facts

| Key | Value |
|---|---|
| Language | Go 1.26 |
| Module | `orpheus` |
| TUI Framework | [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) + [Bubbles](https://github.com/charmbracelet/bubbles) |
| AI Backend | Groq API (Llama 3.3 70B) via OpenAI-compatible REST endpoint |
| Config | `.env` file with `GROQ_API_KEY` (and optional `ORPHEUS_MODEL`) |
| Cache | `~/.cache/orpheus/analysis.json` |
| Binary | `orpheus` (built with `go build`) |
| Target OS | Linux (Arch-based, uses `pacman`) |
| Optional Deps | `yay` or `paru` (for AUR search, install & upgrade support), `flatpak` |
| Theme | Gruvbox Dark |

---

## Directory Structure

```
orpheus/
├── main.go                  # Entry point — loads .env, starts Bubble Tea program
├── go.mod / go.sum          # Module definition and dependency lock
├── .env                     # GROQ_API_KEY (not committed)
├── .gitignore
├── GEMINI.md                # This file
├── README.md                # User-facing documentation
│
└── internal/
    ├── ai/
    │   ├── analyzer.go      # Groq/Llama AI integration (singleflight, backoff, retry logic)
    │   └── analyzer_test.go # Deduplication & delay tests
    │
    ├── cache/
    │   ├── cache.go         # Thread-safe JSON file cache with RWMutex & multi-key matching
    │   └── cache_test.go    # Cache lookup unit tests
    │
    ├── pm/                  # Package Manager abstraction layer
    │   ├── package.go       # Package struct, Manager interface
    │   ├── pacman.go        # Pacman/AUR implementation (pacman -Qi parser, search, update, orphans)
    │   ├── flatpak.go       # Flatpak implementation (list, search, install, update, cleanup)
    │   ├── fuzzy.go         # Fuzzy search and repository relevance ranking engine
    │   └── fuzzy_test.go    # Fuzzy scoring unit tests
    │
    └── tui/                 # Terminal UI (Bubble Tea)
        ├── model.go         # Model struct, Init(), tea commands, helpers
        ├── msgs.go          # Tea message types
        ├── update.go        # Update() — key handling, state transitions, modals, execution logic
        ├── view.go          # View() — rendering sidebar, list, detail, modals, status bar
        └── styles.go        # Lip Gloss color constants and style definitions
```

---

## Architecture

### 1. Entry Point (`main.go`)

- Custom `loadEnv()` that reads `.env` relative to the executable's path (not CWD).
  Parses `KEY=VALUE` lines, skips blanks and `#` comments. Only sets env vars not already set.
- Creates `tui.New()` model, wraps in `tea.NewProgram` with `tea.WithAltScreen()` and `tea.WithMouseCellMotion()`.
- Runs the Bubble Tea event loop.

### 2. Package Manager Layer (`internal/pm/`)

#### `Manager` Interface (`package.go`)

```go
type Manager interface {
    Name() string
    ListAll() ([]Package, error)
    GetPackage(name string) (*Package, error)
    UninstallCmd(names []string) []string
    UninstallOrphansCmd() []string
    GetOrphans() ([]string, error)
    InstallCmd(name string) []string
    UpdateCmd() []string
    UpdatePackagesCmd(names []string) []string
    Search(query string) ([]Package, error)
    RequiresSudo() bool
}
```

#### `Package` Struct

```go
type Package struct {
    Name          string
    Version       string
    Description   string
    Architecture  string
    Size          int64         // bytes
    InstallDate   time.Time
    BuildDate     time.Time
    InstallReason string        // e.g. "Explicitly installed"
    Dependencies  []string
    OptDeps       []string      // Optional dependencies
    OptFor        []string      // "Optional For"
    HasService    bool          // (unused — services removed)
    ServiceName   string        // (unused)
    ServiceStatus string        // (unused)
    IsSystem      bool          // True for base/base-devel deps
    Repository    string        // e.g. "core", "extra", "aur", "flathub"
    IsInstalled   bool          // True if package is already installed
}
```

Helper methods: `SizeMB() float64`, `FormatSize() string` (human-readable: B/KiB/MiB/GiB).

#### Implementations

| Manager | `Name()` | `ListAll()` | `InstallCmd()` | `UpdateCmd()` | `UninstallCmd()` |
|---|---|---|---|---|---|
| **Pacman** | `"pacman"` | `pacman -Qi` → full parser, marks base/base-devel as `IsSystem` | `["<helper>", "-S", "--noconfirm", name]` | `["<helper>", "-Syu", "--noconfirm"]` | `["pacman", "-Rns", "--noconfirm", ...names]` |
| **Flatpak** | `"flatpak"` | `flatpak list --app --columns=...` | `["sh", "-c", "dbus-run-session flatpak install -y " + name]` | `["sh", "-c", "dbus-run-session flatpak update -y"]` | `["sh", "-c", "dbus-run-session flatpak uninstall -y --delete-data ... && dbus-run-session flatpak uninstall -y --unused"]` |

#### Relevance & Fuzzy Ranking Engine (`fuzzy.go`)

- `FuzzyScore(target, query)`: Scores exact matches, prefix matches, word token matches, and subsequence fuzzy matches.
- `RankSearchResults(packages, query)`: Ranks candidate packages by fuzzy score and repository priority (`[core]` > `[extra]` > `[multilib]` > `[aur]`).

### 3. AI Analysis (`internal/ai/analyzer.go`)

- Uses **Groq API** at `https://api.groq.com/openai/v1/chat/completions`.
- Default model: `llama-3.3-70b-versatile` (configurable via `ORPHEUS_MODEL` env var).
- API key read from `GROQ_API_KEY` env var.
- HTTP client with 30s timeout.
- **Singleflight Concurrency**: Collapses concurrent identical requests into a single in-flight call using `singleflightGroup`.
- **Circuit Breaker**: 30-second cooldown on HTTP 429 rate limits (`rateLimitedUntil`).
- **Retry Backoff Floor**: Exponential backoff with a minimum 3s floor (`minRetryDelay`) preventing millisecond retry loops.

#### `Analyze(ctx, pkg, explicitNames)`:

- System prompt: "You are a Linux package analyzer. Give concise, honest analysis. No markdown headers or bullet points. Plain text only."
- `max_tokens: 300`, plain text response.
- Injects the full list of explicit package names into prompt context to infer relationships.
- Extracts terminal launch command (`Command: ...`) and safety verdict (`[SAFE]`, `[CAUTION]`, `[KEEP]`).

### 4. Cache (`internal/cache/cache.go`)

- Thread-safe via `sync.RWMutex`.
- Path: `~/.cache/orpheus/analysis.json`.
- `GetPackage(name, version)`: Checks both `"name@version"` and `"name"` fallback.
- `Has(name, version)`: Returns true if the package analysis exists in cache under either format.
- `Set(key, value)`: Inserts and persists indented JSON.

### 5. Terminal UI (`internal/tui/`)

#### Keybindings

| Key | Context | Action |
|---|---|---|
| `j` / `k` | Global / List / Detail | Navigate down / up |
| `h` / `l` | Panels | Switch panel focus (Sidebar ↔ List ↔ Detail) |
| `Space` | List | Toggle selection for package under cursor |
| `v` | List | Enter / commit visual range selection mode |
| `Enter` / `l` | List | Open package detail |
| `x` | List / Detail | Remove highlighted or multi-selected package(s) |
| `i` | Global | Open package search & installation modal |
| `u` | List / Detail | Update selected package(s) |
| `U` | List / Detail / Sidebar | **Full Upgrade** for the active package manager |
| `o` | Global | Check and batch-clean orphan packages |
| `/` | List | Filter installed packages |
| `s` | List | Cycle sort: Name → Size → Date |
| `r` | List | Reload package list |
| `q` | Global | Quit Orpheus |

#### Modals & Flows
1. **Search & Install Modal (`i`)**: Interactive search input, results list with repository tags (`[core]`, `[extra]`, `[aur]`), package preview (`Tab`), and sudo authentication.
2. **Full Upgrade & Package Update Modal (`U` / `u`)**: Password prompt for sudo managers, execution spinner, and real-time scrollable output log.
3. **Orphan Cleanup Modal (`o`)**: Scans `pacman -Qtdq`, displays count, prompts for password, and uninstalls orphans.
4. **Package Removal Flow (`x`)**: In-detail password prompt rendered at the top of the panel with live asterisk feedback and automatic cleanup.

---

## Build & Test

```bash
# Run tests
go test -v ./...

# Build binary
go build -o orpheus .

# Run
./orpheus
```
