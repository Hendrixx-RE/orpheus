# Packichu — Project Reference for AI Agents

> **Packichu** is a terminal-based (TUI) package management and AI-powered system inspection dashboard for Arch Linux.
> It lets users browse, inspect, AI-analyze, search, install, update, and batch-uninstall packages across multiple
> package managers (Pacman, AUR, Flatpak) in a unified, reactive interface.

---

## Quick Facts

| Key | Value |
|---|---|
| Language | Go 1.26 |
| Module | `packichu` |
| TUI Framework | [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) + [Bubbles](https://github.com/charmbracelet/bubbles) |
| AI Providers | **Google Gemini** (`gemini-2.5-flash`), **Groq** (`openai/gpt-oss-120b`), **OpenAI** (`gpt-4o-mini`), **Anthropic** (`claude-3-5-haiku-latest`) |
| Provider Detection | Automatic based on active API key in config (`GEMINI_API_KEY`, `GROQ_API_KEY`, etc.) |
| Config | `~/.config/packichu/config.env` (or `~/.config/packichu/.env`, `./.env`, `~/.packichu.env`) |
| Cache | `~/.cache/packichu/analysis.json` (keyed permanently by `pkg.Name`) |
| Binary | `packichu` (built with `go build`) |
| Target OS | Linux (Arch-based, uses `pacman`) |
| Optional Deps | `yay` or `paru` (for AUR search, install & upgrade support), `flatpak` |
| Color Themes | **Gruvbox Retro** (Default), **Catppuccin** (Mocha), **Monokai** (cycle with `t`) |

---

## Directory Structure

```
packichu/
├── main.go                  # Entry point — loads XDG config, starts Bubble Tea program
├── go.mod / go.sum          # Module definition and dependency lock
├── .env.example             # Template config file with Gemini, Groq, OpenAI, Anthropic examples
├── .gitignore
├── Makefile                 # Build, test, clean, install targets
├── aur/
│   └── PKGBUILD             # Arch Linux User Repository (AUR) package build definition
├── GEMINI.md                # AI Agent Reference & Architecture Guide (this file)
├── README.md                # User-facing documentation
│
└── internal/
    ├── ai/
    │   ├── analyzer.go      # Multi-provider AI integration (auto-detection, singleflight, pacing, backoff)
    │   └── analyzer_test.go # Deduplication, delay floor & provider auto-detection tests
    │
    ├── cache/
    │   ├── cache.go         # Thread-safe JSON file cache (RWMutex, package-name keying, auto-migration)
    │   └── cache_test.go    # Cache lookup, version persistence & migration tests
    │
    ├── pm/                  # Package Manager abstraction layer
    │   ├── package.go       # Package struct, Manager interface
    │   ├── detect.go        # Dynamic host package manager detection (Pacman, AUR, Flatpak)
    │   ├── detect_test.go   # Manager detection unit tests
    │   ├── pacman.go        # Pacman implementation (pacman -Qin parser, search, update, orphans)
    │   ├── aur.go           # AUR implementation (pacman -Qim, yay/paru helper search & install)
    │   ├── flatpak.go       # Flatpak implementation (list, search, install, update, cleanup)
    │   ├── fuzzy.go         # Fuzzy search and repository relevance ranking engine
    │   └── fuzzy_test.go    # Fuzzy scoring unit tests
    │
    └── tui/                 # Terminal UI (Bubble Tea)
        ├── model.go         # Model struct, Init(), tea commands, theme helpers
        ├── msgs.go          # Tea message types
        ├── update.go        # Update() — key handling, cross-manager sync worker, modal flows, upgrades
        ├── view.go          # View() — rendering sidebar, list, detail, highlighted command badge, modals
        ├── styles.go        # Theme definitions (Gruvbox Retro, Catppuccin, Monokai) & ApplyTheme()
        ├── theme_test.go    # Theme cycling & palette tests
        └── verdict_test.go  # Verdict extraction, command badge cleaning & markdown parsing tests
```

---

## Architecture

### 1. Entry Point & Config Loading (`main.go`)

- `loadConfig()` searches and loads configuration in standard priority order:
  1. `~/.config/packichu/config.env` (or `~/.config/packichu/.env`, `~/.config/packichu/config`)
  2. `~/.packichu.env`
  3. `./.env` (Current working directory)
  4. Executable directory `.env`
- Parses `KEY=VALUE` pairs, strips comments/quotes, and sets environment variables not already exported.
- Launches the Bubble Tea program with full alt-screen and mouse support.

---

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
    InstallReason string        // "Explicitly installed" or "Installed as a dependency"
    Dependencies  []string
    OptDeps       []string      // Optional dependencies
    OptFor        []string      // "Optional For"
    IsSystem      bool          // True for base/base-devel dependencies
    Repository    string        // e.g. "core", "extra", "multilib", "aur", "flathub"
    IsInstalled   bool          // True if package is already installed
}
```

#### Implementations

| Manager | `Name()` | `ListAll()` | `InstallCmd()` | `UpdateCmd()` | `UninstallCmd()` |
|---|---|---|---|---|---|
| **Pacman** | `"pacman"` | `pacman -Qin` → native packages, marks base/base-devel as `IsSystem` | `["pacman", "-S", "--noconfirm", name]` | `["pacman", "-Syu", "--noconfirm"]` | `["pacman", "-Rns", "--noconfirm", ...names]` |
| **AUR** | `"aur"` | `pacman -Qim` → foreign/AUR packages | `["yay", "-S", "--noconfirm", "--sudoloop", "--answerclean=None", "--answerdiff=None", ...]` | `["yay", "-Sua", "--noconfirm", "--sudoloop", ...]` | `["pacman", "-Rns", "--noconfirm", ...names]` |
| **Flatpak** | `"flatpak"` | `flatpak list --app --columns=...` | `["sh", "-c", "dbus-run-session flatpak install -y --or-update --system " + name + " || ..."]` | `["sh", "-c", "dbus-run-session flatpak update -y"]` | `["sh", "-c", "dbus-run-session flatpak uninstall -y --delete-data ... && dbus-run-session flatpak uninstall -y --unused"]` |

#### Relevance & Fuzzy Ranking Engine (`fuzzy.go`)

- `FuzzyScore(target, query)`: Scores exact matches, prefix matches, word token matches, and subsequence fuzzy matches.
- `RankSearchResults(query, packages)`: Ranks candidate packages by fuzzy score and repository priority (`[core]` > `[extra]` > `[multilib]` > `[aur]`).

---

### 3. AI Analysis & Rate-Limit Engine (`internal/ai/analyzer.go`)

- **Supported Providers**:
  - **Gemini**: `gemini-2.5-flash` via Google AI Studio OpenAI-compatible endpoint.
  - **Groq**: `openai/gpt-oss-120b` via Groq Cloud API.
  - **OpenAI**: `gpt-4o-mini` via OpenAI API.
  - **Anthropic**: `claude-3-5-haiku-latest` via Anthropic Messages API.
- **Auto-Detection**: Scans active keys (`GEMINI_API_KEY`, `GROQ_API_KEY`, etc.) and automatically binds provider and default model without manual flags.
- **Reasoning Headroom**: `max_tokens: 2048` (and `1024` for summaries) preventing "thinking" token starvation.
- **Thinking Filter**: `stripThinking()` removes `<think>...</think>` blocks from reasoning model output.
- **Singleflight Deduplication**: Collapses concurrent identical analysis calls into a single in-flight network request keyed by `pkg.Name`.
- **Proactive Interval Pacing**:
  - Gemini: $4,000\text{ms}$ pacing ($15\text{ RPM}$, matching free-tier limits).
  - Groq / OpenAI / Anthropic: $2,500\text{ms}$ pacing ($24\text{ RPM}$).
- **Circuit Breaker**: 30-second cooldown on HTTP 429 (`rateLimitedUntil`).
- **Retry Backoff Floor**: Minimum 3-second delay floor on HTTP 429 retries.

---

### 4. Cache Architecture (`internal/cache/cache.go`)

- **Thread-safe** via `sync.RWMutex`.
- **Location**: `~/.cache/packichu/analysis.json`.
- **Package-Name Keying**: Analyses are stored directly under `pkg.Name` (e.g. `"neovim"`, `"git"`), making them immune to version upgrades (`pacman -Syu`).
- **Auto-Migration**: Automatically promotes legacy `pkg@version` keys to package-name keys on startup.
- **Manual Invalidation**: Pressing **`a`** forces a fresh re-analysis and updates the cache record.

---

### 5. Terminal UI & Themes (`internal/tui/`)

#### Color Themes (`styles.go`)

Packichu supports 3 complete, hot-swappable color palettes. Pressing **`t`** cycles themes live:

1. **`Gruvbox Retro`** *(Default)*: Base `#282828`, Surface `#3c3836`, Yellow `#fabd2f`, Green `#b8bb26`, Cyan `#8ec07c`, Orange `#fe8019`.
2. **`Catppuccin`** *(Mocha)*: Base `#1e1e2e`, Surface `#313244`, Mauve `#cba6f7`, Sky `#89dceb`, Green `#a6e3a1`, Peach `#fab387`.
3. **`Monokai`**: Base `#272822`, Surface `#3e3d32`, Cyan `#66d9ef`, Pink `#f92672`, Green `#a6e22e`, Purple `#ae81ff`.

#### Cross-Manager Background Auto-Analysis Loop (`runAllManagersSyncWorker`)

- Enumerates all explicitly installed packages across all available managers (`Pacman`, `AUR`, `Flatpak`).
- Sequentially analyzes uncached packages in the background respecting rate-limit pacing.
- Displays a unified `[Done / Total]` progress counter on the status bar (e.g. `[124/850]`).
- Uninterrupted by manager tab switching (`Tab` / sidebar `j`/`k`).
- Reloading with **`r`** resets the worker to detect newly installed packages.

#### Keybindings

| Key | Context | Action |
|---|---|---|
| `j` / `k` | Global / List / Detail | Navigate down / up (auto-displays package detail on hover) |
| `h` / `l` | Panels | Switch panel focus (Sidebar ↔ List ↔ Detail viewport scroll) |
| `Tab` | Global / List / Detail | Switch active package manager (Pacman ↔ AUR ↔ Flatpak) |
| `Space` | List | Toggle selection for package under cursor |
| `v` | List | Enter / commit visual range selection mode |
| `Enter` / `l` | List | Focus detail panel (scroll view) / commit selection |
| `t` | Global / List / Detail | **Cycle Theme: Gruvbox Retro → Catppuccin → Monokai** |
| `x` | List / Detail | Remove highlighted or multi-selected package(s) |
| `i` | Global | Open package search & installation modal |
| `c` | Global / List / Detail / Sidebar | **Clean Package Cache** for active provider (Pacman, AUR, Flatpak) |
| `u` | List / Detail | Update selected package(s) *(with optional AI Changelog preview `[a]`)* |
| `U` | Global / List / Detail / Sidebar | **Full System Upgrade** across all detected package managers (Pacman + AUR + Flatpak) |
| `o` | Global | Check and batch-clean orphan packages |
| `a` | Detail / List | Force AI re-analysis for highlighted package |
| `f` | List | **Toggle filter: Show Updatable Packages Only ↔ All Packages** |
| `/` | List | Filter installed packages |
| `s` | List | Cycle sort: **Name → Size → Install Date** |
| `r` | List | Reload package list & re-check updatable packages |
| `q` | Global | Quit Packichu |

#### Responsive Breakpoints

- **Large (`≥ 105 cols`, `≥ 24 rows`)**: Full 3-panel flex layout (Sidebar 16% | Package List ~44% | Detail View 40%).
- **Medium (`80–104 cols` or `18–23 rows`)**: 2-panel layout with top tab bar for package managers (`List 50% | Detail 50%`).
- **Compact (`< 80 cols` or `< 18 rows`)**: Single-panel focused mode with top tab bar (`Enter` opens Detail full-screen, `Esc`/`h` returns to List).
- **Minimum Guard (`< 45 cols` or `< 10 rows`)**: Centered window resize warning.

#### Modals & Flows

1. **Search & Install Modal (`i`)**: Interactive search input, results list with repository tags (`[core]`, `[extra]`, `[aur]`), package preview (`Tab`), and sudo authentication.
2. **Full System Upgrade (`U`) & Package Update (`u`)**: Single-password session authentication (`sudo -v`), sequential execution across all package managers, real-time log streaming, and scrollable output viewport.
3. **Orphan Cleanup Modal (`o`)**: Scans `pacman -Qtdq`, displays count, prompts for password, and batch-uninstalls orphans.
4. **Package Removal Flow (`x`)**: In-detail password prompt rendered at the top of the panel with live asterisk feedback and automatic cleanup.

---

## Configuration

Packichu loads configuration and API keys from `~/.config/packichu/config.env` (or `~/.config/packichu/.env`, `./.env`, `~/.packichu.env`).

### Example `~/.config/packichu/config.env`:

```env
# Packichu auto-detects whichever key you provide!
# Option 1: Google Gemini (Recommended - Free tier at https://aistudio.google.com)
GEMINI_API_KEY=AIzaSy...your_key_here

# Option 2: Groq (Free tier at https://console.groq.com)
# GROQ_API_KEY=your_groq_key_here

# Option 3: OpenAI
# OPENAI_API_KEY=your_openai_key_here

# Option 4: Anthropic
# ANTHROPIC_API_KEY=your_anthropic_key_here

# Optional: Override the model (defaults to gemini-2.5-flash for Gemini, openai/gpt-oss-120b for Groq)
# PACKICHU_MODEL=gemini-2.5-flash
```

---

## Build & Test

```bash
# Run test suite
make test

# Build binary
make build

# Install locally to /usr/local/bin
sudo make install

# Run
./packichu
```
