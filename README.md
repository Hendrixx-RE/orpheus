
<div align="center">

# Packichu

**A terminal-based package management & AI system inspection dashboard for Arch Linux**

*Browse, inspect, AI-analyze, search, install, update, and batch-uninstall packages across multiple package managers (Pacman, AUR, Flatpak) — all from one unified, reactive TUI.*

https://github.com/user-attachments/assets/3e0c4ca7-8136-41c5-899f-b20bac0c07c1

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Bubble Tea](https://img.shields.io/badge/Bubble_Tea-TUI-FF75B5?style=flat-square)](https://github.com/charmbracelet/bubbletea)
[![License](https://img.shields.io/badge/License-MIT-fabd2f?style=flat-square)](#license)

</div>

---

## Why Packichu?

Over time, an Arch system accumulates hundreds of packages. Some were installed months ago and forgotten; some are multi-gigabyte build toolchains used once; others are mysterious dependencies where it's unclear if removing them will break your desktop environment.

**Packichu gives you total clarity and control.** It scans your system across **Pacman, AUR, and Flatpak**, analyzes each package with your choice of AI provider (Google Gemini, Groq, OpenAI, Anthropic) in the background, and gives you fast, keyboard-driven workflows to install, update, clean caches, remove orphans, and batch-uninstall safely.

---

## Features

### Responsive Three-Panel Dashboard
- **Sidebar (Left)**: Switch between package managers (Pacman, AUR, Flatpak), view total package counts, and check pending update counts per provider.
- **Package List (Middle)**: Real-time filtering, virtual scrolling, selection checkmarks, update indicators (`▲`), and formatted package sizes.
- **Detail Panel (Right)**: Comprehensive metadata (version, architecture, install reason, explicit vs. dependency status, size, dependencies, optional dependencies), highlighted terminal launch commands, safety verdicts, and full AI analysis.
- **Adaptive Layout**: Automatically switches between a 3-Panel view (`≥ 105 cols`), 2-Panel view (`80–104 cols`), and Compact 1-Panel focus view (`< 80 cols`).

---

### Multi-Manager Operations (Auto-Detected)
Packichu automatically detects available package managers on your host system:

| Manager | Source | Install / Search | Full Upgrade (`U`) | Cache Clean (`c`) | Uninstall Strategy |
|---|---|---|---|---|---|
| **Pacman** | Native Arch official repos (`[core]`, `[extra]`, `[multilib]`) | Official Repositories | `pacman -Syu --noconfirm` | `pacman -Sc --noconfirm` | `pacman -Rns --noconfirm` (recursive dependency removal) |
| **AUR** | Foreign / Arch User Repository (`pacman -Qim`) | AUR via `yay` or `paru` | `yay -Sua` / `paru -Sua` | `yay -Sc` / `paru -Sc` | `pacman -Rns --noconfirm` |
| **Flatpak** | User & system Flatpak apps | Flathub & configured remotes | `flatpak update -y` | `flatpak uninstall --unused -y` | Deletes app data + removes unused runtimes |

---

### Context-Aware AI Package Intelligence
Packichu uses your configured AI provider to analyze your software ecosystem in the background:
- **System Context & Purpose**: Explains what the package is and *why* it is on your machine based on other installed software (e.g., detecting `neovim` to infer why `lua51` or `tree-sitter` exists).
- **Safety Verdict**: Clear, color-coded removal safety badges:
  - `[SAFE TO REMOVE]` — Standalone utility, safe to remove if unused.
  - `[CAUTION]` — Has optional dependents or user configurations.
  - `[CRITICAL / KEEP]` — System component, kernel driver, or critical dependency.
- **Highlighted Launch Command Badge**: Automatically parses the primary executable command (e.g., `Command: btop`) for quick reference.
- **Automated Background Scanner**: Continuously analyzes uncached explicit packages in the background across all managers with interval pacing and status bar progress counters (`[Done/Total]`).
- **Singleflight Deduplication & Persistent Cache**: Prevents duplicate API requests; cache is permanently keyed by package name in `~/.cache/packichu/analysis.json` and preserved across package upgrades.

---

### Deep Context-Aware Knowledge Search (`/`)
Press **`/`** in the package list to instantly search installed packages with ranked matching:
1. **Rank 1**: Exact package name matches
2. **Rank 2**: Package name prefix matches
3. **Rank 3**: Package name substring matches
4. **Rank 4**: Package description matches
5. **Rank 5 (Deep Semantic Search)**: Searches directly through the **AI analysis text** and explanation knowledge base (e.g. searching *"image editor"*, *"audio plugin"*, or *"wayland clipboard"* will match relevant packages even if the search term does not appear in the package name).

---

### Interactive Fuzzy Search & Install Modal (`i`)
- Press **`i`** from anywhere to open the package search & installation modal.
- Multi-token fuzzy matching with repository weighting (`[core]` > `[extra]` > `[multilib]` > `[aur]`).
- Press **`Tab`** to toggle package descriptions and generate on-the-fly AI quick summaries before installing.
- Direct non-interactive installation with sudo privilege handling for official repos, AUR helpers, and Flatpak.

---

### Update Center & AI Changelog Previews (`f`, `u`, `U`)
- **Round-Trip Update Detection**: Checks for available updates across all managers on startup and displays badges/counters (e.g. `[1] Pacman (8)`).
- **Updates-Only Filter (`f`)**: Press **`f`** to toggle the list view between *All Packages* and *Updates Available Only* for explicitly installed software.
- **Targeted Package Update (`u`)**: Update highlighted or multi-selected packages with a single confirmation.
- **AI Changelog Summary (`[a]` inside update modal)**: Before confirming an update, press **`a`** to ask AI for a concise release summary of what's new, performance improvements, bug fixes, and potential breaking changes between your installed version and the target version.
- **Full System Upgrade (`U`)**: Press **`U`** to run a full upgrade across all detected package managers (Pacman + AUR + Flatpak) with single sudo password caching and scrollable output log streaming.

---

### Provider-Specific Cache Cleaner (`c`)
- Press **`c`** to open the cache cleaner modal for your active package manager:
  - **Pacman**: Cleans old package tarballs from `/var/cache/pacman/pkg/` (`pacman -Sc`).
  - **AUR**: Cleans build cache, cached git clones, and downloaded source archives for `yay` or `paru`.
  - **Flatpak**: Uninstalls unused runtimes and cleans leftover application cache.

---

### Orphan Package Detection & Batch Removal (`o`)
- Press **`o`** to inspect unused orphan packages (`pacman -Qtdq`).
- View orphan package counts and batch-remove all orphans in a single transaction.

---

### Visual Range Selection & Batch Uninstall (`x`, `v`, `Space`)
- **`Space`**: Toggle selection for individual packages.
- **`v`**: Enter **Visual Range Selection Mode** — highlight contiguous package blocks using `j`/`k`.
- **`x`**: Batch-uninstall all selected packages with an in-panel non-destructive password prompt and live status feedback.

---

### Live Theme Cycling (`t`)
Press **`t`** at any time to instantly cycle through 3 themes:
1. **Gruvbox Retro** *(Default)* — Warm retro tones with yellow, green, and orange accents.
2. **Catppuccin Mocha** — Soft pastel palette with mauve, sky, and peach highlights.
3. **Monokai** — Vibrant high-contrast cyan, pink, and purple styling.

---

## Keybindings

### Navigation & Panels
| Key | Context | Action |
|---|---|---|
| `j` / `k` / `↓` / `↑` | Global / List / Detail | Navigate down / up (auto-displays package detail on hover) |
| `h` / `l` / `←` / `→` | Panels | Switch panel focus (Sidebar ↔ List ↔ Detail scroll) |
| `Ctrl+D` / `Ctrl+U` | List / Detail / Modals | Half-page down / up |
| `G` / `gg` | List / Detail / Modals | Jump to bottom / jump to top |
| `Tab` | Global | Switch active package manager (Pacman ↔ AUR ↔ Flatpak) |
| `Enter` / `l` | List | Focus detail panel (scroll view) / commit selection |
| `Esc` | Global | Clear visual selection / cancel modal / back to list |
| `q` | Global | Quit Packichu |

### Selection, Sorting & Filtering
| Key | Context | Action |
|---|---|---|
| `Space` | List | Toggle selection for package under cursor |
| `v` | List | Enter / commit visual range selection mode |
| `f` | List | **Toggle filter: Show Updatable Packages Only ↔ All Packages** |
| `/` | List | Live search & deep AI semantic filter |
| `s` | List | Cycle sort: **Name → Size → Install Date** |
| `t` | Global | **Cycle Theme: Gruvbox Retro → Catppuccin → Monokai** |
| `r` | List | Reload package list & re-check updatable packages |

### Actions & Modals
| Key | Context | Action |
|---|---|---|
| `i` | Global | Open package search & installation modal |
| `u` | List / Detail | Update selected package(s) *(with optional AI Changelog preview `[a]`)* |
| `U` | Global | **Full System Upgrade** across all package managers (Pacman + AUR + Flatpak) |
| `c` | Global | **Clean Package Cache** for active package manager (Pacman, AUR, Flatpak) |
| `o` | Global | Check and batch-clean orphan packages |
| `x` | List / Detail | Remove highlighted or multi-selected package(s) |
| `a` | Detail / List | Force AI re-analysis for highlighted package |

---

## Installation

### Prerequisites
- **Go 1.26+**

### Build from Source
```bash
git clone https://github.com/Hendrixx-RE/packichu.git
cd packichu
go build -ldflags="-s -w" -o packichu .
```

To install system-wide:
```bash
sudo make install
```

---

## Configuration

Packichu searches and loads configuration in standard priority order:
1. `~/.config/packichu/config.env` (or `~/.config/packichu/.env`, `~/.config/packichu/config`)
2. `~/.packichu.env`
3. `./.env` (Current working directory)

### Supported Providers & Automatic Detection

Packichu automatically binds the provider and model based on whichever API key is present:

| Provider | Config Key | Default Model | Free Tier? |
|---|---|---|:---:|
| **Google Gemini** *(Recommended)* | `GEMINI_API_KEY` | `gemini-2.5-flash` | Yes ([Google AI Studio](https://aistudio.google.com)) |
| **Groq** | `GROQ_API_KEY` | `openai/gpt-oss-120b` | Yes ([Groq Console](https://console.groq.com)) |
| **OpenAI** | `OPENAI_API_KEY` | `gpt-4o-mini` | Paid API credits |
| **Anthropic** | `ANTHROPIC_API_KEY` | `claude-3-5-haiku-latest` | Paid API credits |

### Example `~/.config/packichu/config.env`:

```env
# Packichu auto-detects whichever key you provide!

# Option 1: Google Gemini (Recommended - Free tier at https://aistudio.google.com)
GEMINI_API_KEY=AIzaSy...your_key_here

# Option 2: Groq (Free tier at https://console.groq.com)
# GROQ_API_KEY=gsk_...your_key_here

# Option 3: OpenAI
# OPENAI_API_KEY=sk-...your_key_here

# Option 4: Anthropic
# ANTHROPIC_API_KEY=sk-ant-...your_key_here

# Optional: Override the default model
# PACKICHU_MODEL=gemini-2.5-flash

# Optional: Force a specific provider if multiple keys exist (gemini, groq, openai, anthropic)
# PACKICHU_PROVIDER=gemini

# Optional: Custom pacing delay between background requests (default: 4s for Gemini, 2.5s for others)
# PACKICHU_RATE_LIMIT_DELAY=4s
```

---

## Architecture

```
packichu/
├── main.go                  # Entry point — loads XDG config, starts Bubble Tea program
├── go.mod / go.sum          # Module definition & dependency locks
├── .env.example             # Template configuration file
├── Makefile                 # Build, test, clean, install targets
├── aur/
│   └── PKGBUILD             # Arch User Repository build script
├── GEMINI.md                # Agent reference and architecture guide
├── README.md                # User documentation
│
└── internal/
    ├── ai/
    │   ├── analyzer.go      # Multi-provider client (Gemini, Groq, OpenAI, Anthropic), singleflight, pacing
    │   └── analyzer_test.go # Deduplication, delay floor, changelog & provider detection tests
    │
    ├── cache/
    │   ├── cache.go         # Thread-safe JSON cache by package name (sync.RWMutex, auto-migration)
    │   └── cache_test.go    # Cache lookup and version upgrade retention tests
    │
    ├── pm/                  # Package Manager Abstraction Layer
    │   ├── package.go       # Package struct & Manager interface (CleanCacheCmd, InstallCmd, etc.)
    │   ├── detect.go        # Dynamic host package manager detection (Pacman, AUR, Flatpak)
    │   ├── pacman.go        # Pacman implementation (native packages, checkupdates, cache clean, search)
    │   ├── aur.go           # AUR foreign implementation (yay/paru integration, cache clean)
    │   ├── flatpak.go       # Flatpak implementation (list, search, install, update, unused cleanup)
    │   ├── fuzzy.go         # Multi-token fuzzy matching & repository relevance ranking
    │   └── fuzzy_test.go    # Fuzzy scoring test suite
    │
    └── tui/                 # Terminal UI (Bubble Tea + Lip Gloss)
        ├── model.go         # State machine, model definitions, async commands, filter/search engine
        ├── msgs.go          # Bubble Tea message types
        ├── update.go        # Key handlers, cross-manager sync worker, modals, full upgrade loop
        ├── view.go          # Responsive layouts (sidebar, list, detail, highlighted command badge, modals)
        ├── styles.go        # Multi-theme design system (Gruvbox Retro, Catppuccin, Monokai)
        ├── theme_test.go    # Theme cycling tests
        ├── verdict_test.go  # Verdict and command parsing tests
        └── updatable_test.go# Updates filter, AI preview & cache clean modal tests
```

---

## Contributing

Contributions are welcome! Adding a new package manager is straightforward — implement the `pm.Manager` interface in `internal/pm/`:

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
    GetUpdatable() ([]UpdatablePackage, error)
    CleanCacheCmd() []string
    Search(query string) ([]Package, error)
    RequiresSudo() bool
}
```

---

## License

MIT License. See [LICENSE](LICENSE) for details.

---

<div align="center">

**Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss)**

*Packichu — Package management made clean, fast, and intelligent.*

</div>
