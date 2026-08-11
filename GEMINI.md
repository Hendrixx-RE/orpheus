# Orpheus — Project Reference for AI Agents

> **Orpheus** is a terminal-based (TUI) package management dashboard for Arch Linux.
> It lets users browse, inspect, AI-analyze, and batch-uninstall packages from multiple
> package managers (Pacman, npm) in a single unified interface.

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
| Optional Deps | `yay` or `paru` (for AUR search & install support) |
| Theme | Gruvbox Dark |

---

## Directory Structure

```
orpheus/
├── main.go                  # Entry point — loads .env, starts Bubble Tea program
├── go.mod / go.sum          # Module definition and dependency lock
├── .env                     # GROQ_API_KEY (not committed)
├── .gitignore
├── todolist.md              # Planned features
├── GEMINI.md                # This file
│
└── internal/
    ├── ai/
    │   └── analyzer.go      # Groq/Llama AI integration for package analysis
    │
    ├── cache/
    │   └── cache.go         # Thread-safe JSON file cache for AI analysis results
    │
    ├── pm/                  # Package Manager abstraction layer
    │   ├── package.go       # Package struct, Manager interface
    │   ├── pacman.go        # Pacman implementation (pacman -Qi parser)
    │   └── npm.go           # npm implementation (inline Node.js script)
    │
    └── tui/                 # Terminal UI (Bubble Tea)
        ├── model.go         # Model struct, Init(), tea commands, helpers
        ├── msgs.go          # Tea message types
        ├── update.go        # Update() — key handling, state transitions, all logic
        ├── view.go          # View() — rendering sidebar, list, detail, help bar
        └── styles.go        # Lip Gloss color constants and style definitions
```

---

## Architecture

### 1. Entry Point (`main.go` — 62 lines)

- Custom `loadEnv()` that reads `.env` relative to the executable's path (not CWD).
  Parses `KEY=VALUE` lines, skips blanks and `#` comments. Only sets env vars not already set.
- Creates `tui.New()` model, wraps in `tea.NewProgram` with `tea.WithAltScreen()` and
  `tea.WithMouseCellMotion()`.
- Runs the Bubble Tea event loop.

### 2. Package Manager Layer (`internal/pm/`)

#### `Manager` Interface (`package.go`)

```go
type Manager interface {
    Name() string                              // e.g. "pacman", "node"
    ListAll() ([]Package, error)               // List all relevant packages
    GetPackage(name string) (*Package, error)  // Get detailed info for one package
    UninstallCmd(names []string) []string       // Build uninstall command (batch)
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
}
```

Helper methods: `SizeMB() float64`, `FormatSize() string` (human-readable: B/KiB/MiB/GiB).

#### Implementations

| Manager | `Name()` | `ListAll()` | `UninstallCmd()` |
|---|---|---|---|
| **Pacman** | `"pacman"` | `pacman -Qi` → full parser, marks base/base-devel as `IsSystem` | `["pacman", "-Rns", "--noconfirm", ...names]` |
| **Npm** | `"node"` | Inline `node -e` script: reads global `node_modules`, recursively calculates dir sizes | `["npm", "uninstall", "-g", ...names]` |

#### Pacman Parser Details (`pacman.go` — 238 lines)

- `parsePacmanQi(data)`: State machine parser for `pacman -Qi` output. Handles multi-line
  field continuation (lines starting with space/tab). Uses `finalize()` closure to accumulate packages.
- `appendField()`: Handles continuation lines for multi-value fields (Depends On, Optional Deps, etc.).
- `parseOptDepLine()`: Extracts dep name from lines like `"python: Python support [installed]"`.
- `parseSize()`: Handles GiB, MiB, KiB, and raw bytes.
- `parseDate()`: Tries 3 date formats common in pacman output.
- `GetOrphansDetailed()`: Standalone exported function (not on Pacman receiver). Runs `pacman -Qdti`.
- `runCmd()` / `runCmdAllowExit1()`: Shared command execution helpers.

> **Design Decision — Pacman**: Only **explicitly installed** packages are shown. Filtering
> happens in `applyFilter()` in `model.go` which checks `InstallReason == "Explicitly installed"`.

> **Design Decision — npm**: Size is calculated by recursively walking each package's directory
> under the global `node_modules` root, since `npm list` doesn't report sizes natively.

> **Removed — Pip**: Was previously implemented but removed because pip cannot install packages
> system-wide on modern Arch (PEP 668 externally-managed). The file no longer exists.

### 3. AI Analysis (`internal/ai/analyzer.go` — 174 lines)

- Uses **Groq API** at `https://api.groq.com/openai/v1/chat/completions`.
- Default model: `llama-3.3-70b-versatile` (configurable via `ORPHEUS_MODEL` env var).
- API key read from `GROQ_API_KEY` env var.
- HTTP client with 30s timeout.
- **Retry logic**: Up to 4 attempts with exponential backoff (5s → 10s → 20s → 40s).
  Only retries on HTTP 429 (rate limit). Respects context cancellation between retries.

#### `Analyze(ctx, pkg, explicitNames)`:

- System prompt: "You are a Linux package analyzer. Give concise, honest analysis.
  No markdown headers or bullet points. Plain text only."
- `max_tokens: 300`, no temperature specified in request body.
- User prompt includes:
  - Package name, version, description, install reason, install date, size.
  - Up to 8 dependencies.
  - **The full list of all explicitly installed package names** — allows the AI to infer
    *why* a package exists on the system.
- Asks the AI: (1) purpose on system, (2) why user installed it given other packages,
  (3) what happens if removed. Returns a verdict: `[SAFE]`, `[CAUTION]`, or `[KEEP]`.

#### Unexported Helpers:
- `extractContent(data)` — Parses OpenAI-compatible JSON response for `choices[0].message.content`.
- `extractError(data)` — Parses `error.message` from API error response.
- `buildPrompt(pkg, explicitNames)` — Constructs the user prompt.

### 4. Cache (`internal/cache/cache.go` — 70 lines)

- Thread-safe via `sync.RWMutex`.
- Path: `os.UserCacheDir()/orpheus/analysis.json` (falls back to `os.TempDir()`).
- `New()` — Creates directory structure, loads existing JSON data.
- `Get(key)` — Read-locked lookup.
- `Set(key, value)` — Write-locked insert + immediate disk persist.
- Key format used by TUI: `"packageName@version"`.
- `load()` silently ignores missing file; `log.Fatal` on malformed JSON.
- `save()` marshals + writes; `log.Fatal` on write failure.

### 5. TUI (`internal/tui/`)

#### Panel Layout

```
┌──────────┬─────────────────────────┬──────────────────────┐
│ Sidebar  │     Package List        │   Detail Panel       │
│ (18ch)   │     (flexible)          │   (46ch)             │
│          │                         │                      │
│ Orpheus  │  ● package-name  1.2MB  │  Name: ...           │
│          │  ✓ selected-pkg  500KB  │  Version: ...        │
│ Packages │  ● another-pkg   200KB  │  AI Analysis: ...    │
│  > pacman│                         │                      │
│    node  │                         │                      │
└──────────┴─────────────────────────┴──────────────────────┘
│                    Help Bar (context-sensitive)            │
└───────────────────────────────────────────────────────────┘
```

#### Message Types (`msgs.go` — 25 lines)

| Type | Fields | Trigger |
|---|---|---|
| `pkgsLoadedMsg` | `pkgs []pm.Package`, `err error` | `loadPackages()` |
| `pkgDetailMsg` | `pkg *pm.Package`, `err error` | `loadPackageDetail()` |
| `aiAnalysisMsg` | `text string`, `err error` | `analyzePackage()` |
| `pkgRemovedMsg` | `err error` | `removePackageCmdAsync()` |

#### Model State (`model.go` — 330 lines)

Key state groups in the `Model` struct:

| Group | Fields | Purpose |
|---|---|---|
| Layout | `width`, `height`, `ready`, `focusedPanel` | Terminal dimensions and focus |
| Packages | `allPkgs`, `filteredPkgs`, `listCursor`, `listOffset`, `loading`, `sortMode` | Package data and list navigation |
| Search | `searching`, `searchInput` | Search filter |
| Detail | `selectedPkg`, `detailVP`, `aiText`, `aiLoading`, `aiErr` | Single package detail view |
| Removal | `askingPassword`, `removingLoading`, `removeErr`, `passwordInput` | Password prompt and removal state |
| Multi-select | `selectedPkgs` (map[string]bool), `visualMode`, `visualStart` | Yazi-style selection |
| Managers | `managers` ([]pm.Manager), `activeMgr` | Package manager switching |
| Services | `spinner`, `lastKey`, `err`, `aiSvc`, `cache` | Shared utilities |

#### Keybindings (`update.go` — 602 lines)

**Global:**
- `ctrl+c` — Quit
- `q` — Quit (unless in search or password mode)

**Sidebar:**

| Key | Action |
|---|---|
| `j/k` | Switch active manager (wraps), reload packages, clear selection |
| `l/Enter` | Focus list panel |

**List Panel:**

| Key | Action |
|---|---|
| `Space` | Toggle single package selection (commits visual first) |
| `v` | Enter/exit visual mode (start/commit range select) |
| `j/k` | Move cursor up/down |
| `Ctrl+d/u` | Half page down/up |
| `G` | Jump to bottom |
| `g g` | Jump to top (double-tap via `lastKey` tracking) |
| `/` | Enter search mode |
| `s` | Cycle sort: name → size → date → name |
| `r` | Reload packages from current manager |
| `Esc` | Cascading: exit visual → clear selections → clear search |
| `Enter/l` | Commit visual selection, then `triggerSelect()` |
| `h` | Focus sidebar |

**Detail Panel:**

| Key | Action |
|---|---|
| `a` | Trigger AI analysis (single package only) |
| `x` | Initiate removal (single or batch) |
| `j/k` | Scroll detail viewport |
| `Ctrl+d/u` | Half-page scroll |
| `g g / G` | Top/bottom of detail |
| `h/Esc` | Focus list panel |

**Password Prompt:**

| Key | Action |
|---|---|
| `Enter` | Submit password, execute removal |
| `Esc` | Cancel |

#### Multi-Selection System

Mirrors Yazi's behavior:

1. **Single toggle** (`Space`): Toggles `selectedPkgs[name]` on/off.
2. **Visual mode** (`v`): Sets `visualStart` to cursor position. Moving `j/k` expands
   the visual range dynamically. Pressing `v` again or `Enter` commits the range to
   `selectedPkgs` map.
3. **Visual highlight**: `isPkgSelected(i)` checks both the `selectedPkgs` map AND
   whether index `i` falls within the active visual range.
4. **Clear**: `Esc` first exits visual mode, then clears all selections, then clears search.
5. **Batch view**: When multiple packages are selected and `Enter` is pressed,
   `triggerSelect()` shows a "Batch Operation" detail view instead of individual package details.

#### Removal Flow

```
x pressed → askingPassword=true → passwordInput focused
  → user types password → Enter
  → removingLoading=true → removePackageCmdAsync(cmdArgs, pw)
  → runs: sudo -S <manager.UninstallCmd(names)>
  → password piped via stdin
  → on success: reloads packages, clears selectedPkgs, focuses list
  → on error: shows removeErr in detail panel
```

Both single and batch removal use the same flow. `UninstallCmd()` accepts `[]string`,
so `pacman -Rns --noconfirm pkg1 pkg2 pkg3` is a single command.

#### Visual Indicators

| Badge | Meaning |
|---|---|
| Cyan `●` | Normal unselected package |
| Purple `✓` | Selected package |
| Yellow `●` | Hovered (cursor) unselected |
| Yellow `✓` | Hovered + selected |

#### Detail Content Builders (`update.go`)

- `buildDetailContent()` — Single package: name+version, description (word-wrapped),
  size, date, reason, architecture, dependencies (up to 8 + "N more"), then "Action Status"
  section with password prompt / spinner / error / AI text+verdict / default hint.
- `buildBatchDetailContent()` — Batch: count, up to 10 names + "...and N more",
  then action status (password/spinner/error/hint).
- `splitVerdict(text)` — Splits AI text at last line starting with "Verdict:".

#### Rendering (`view.go` — 232 lines)

- `renderSidebar()` — "Orpheus" brand, "Packages" header, manager list with `> ` active indicator.
- `renderPackageList()` — Header with count + sort label + search query. Virtual scrolling
  with scroll indicator showing `N/Total X%`.
- `renderPkgLine()` — Badge + truncated name + right-aligned dimmed size.
- `renderStatusBar()` — Context-sensitive keybinding hints.

### 6. Styles (`styles.go` — 54 lines)

Gruvbox Dark color palette:

| Variable | Hex | Usage |
|---|---|---|
| `colorBase` | `#282828` | Background |
| `colorBorder` | `#504945` | Unfocused borders |
| `colorBorderFoc` | `#d79921` | Focused borders (gold) |
| `colorText` | `#ebdbb2` | Primary text |
| `colorMuted` | `#a89984` | Dimmed/secondary text |
| `colorGreen` | `#b8bb26` | Verdict safe text |
| `colorYellow` | `#fabd2f` | Titles, selected items, keybinds |
| `colorRed` | `#fb4934` | Errors, orphan highlighting |
| `colorCyan` | `#8ec07c` | Explicit badges, AI labels |
| `colorPurple` | `#d3869b` | Spinner, selected checkmarks |
| `colorOrange` | `#fe8019` | (defined, not currently used) |

---

## Important Constraints

1. **Explicit packages only**: The entire app philosophy is to show only explicitly installed
   packages. Dependencies are hidden from the main list via `applyFilter()`.
2. **No circular imports**: `pm`, `ai`, `cache`, and `tui` are independent packages.
   `tui` imports `pm`, `ai`, and `cache`. Nothing imports `tui`.
3. **No pip support**: Removed by design. Modern Arch/PEP 668 prevents system-wide pip installs.
4. **Batch uninstall**: `UninstallCmd` accepts `[]string`, not `string`. All managers must
   support multi-package removal in a single command.
5. **Password handling**: Collected via TUI text input, piped to `sudo -S` via stdin.
   Never stored beyond immediate command execution.
6. **AI context**: The AI prompt always receives the full list of explicitly installed package
   names so it can make informed recommendations about package safety.
7. **Thread-safe cache**: The cache uses `sync.RWMutex` for concurrent access safety.
8. **AI retry logic**: Exponential backoff (5s→10s→20s→40s) on HTTP 429 rate limits, up to 4 retries.

---

## Build & Run

```bash
# Build
go build -o orpheus .

# Run
./orpheus

# Or directly
go run .
```

Requires:
- Go 1.26+
- `pacman` (Arch Linux)
- `node` + `npm` (for npm package listing)
- `GROQ_API_KEY` in `.env` file (for AI analysis)

---

## Planned Features (`todolist.md`)

- Support for Rust, Go, pipx package managers
- Install capability for all managers
- Global search
- Update for each package manager
- Cache clear
- Services monitoring (tentative)
