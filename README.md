<div align="center">

# Orpheus

**A terminal-based package management dashboard for Arch Linux**

*Browse, inspect, AI-analyze, search, install, update, and batch-uninstall packages across multiple package managers — all from one unified TUI.*

![Orpheus Dashboard](https://github.com/user-attachments/assets/03d0c46a-2120-4d3a-a6d2-01f45591aeb1)

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Bubble Tea](https://img.shields.io/badge/Bubble_Tea-TUI-FF75B5?style=flat-square)](https://github.com/charmbracelet/bubbletea)
[![License](https://img.shields.io/badge/License-MIT-fabd2f?style=flat-square)](#license)

</div>

---

##  Why Orpheus?

Your system accumulates packages over time. Some you installed months ago and forgot about. Some are 500 MB behemoths you used once. Some are mysterious dependencies you're not even sure what they do anymore.

**Orpheus gives you total clarity and control.** It pulls every explicitly installed package from Pacman/AUR and Flatpak into a single dashboard, lets an AI analyze safety and system context, and provides one-key workflows for installing, updating, orphan cleaning, and batch uninstallation.

---

##  Features

###  Three-Panel Dashboard
A clean, vim-navigable interface:
- **Sidebar (Left)**: Switch between package managers (Pacman, Flatpak) and view high-level counts.
- **Package List (Middle)**: Real-time filtering, virtual scrolling, selection badges, and size formatting.
- **Detail Panel (Right)**: Comprehensive package metadata, dependencies, action status, and AI insights.

###  Multi-Manager Operations
| Manager | What It Lists | Install / Search | Full Upgrade (`U`) | Uninstall Strategy |
|---|---|---|---|---|
| **Pacman** | All explicitly installed packages (`pacman -Qi`) | Official Repos (`[core]`, `[extra]`, `[multilib]`) + **AUR** via `yay`/`paru` | `yay -Syu --noconfirm` / `pacman -Syu` | `pacman -Rns --noconfirm` (recursive dependency removal) |
| **Flatpak** | All installed Flatpak desktop applications | Remote search on configured Flatpak remotes (Flathub, etc.) | `flatpak update -y` | Uninstalls app + deletes user data + cleans unused runtimes |

###  AI-Powered Package Analysis
Press `a` on any package and Orpheus leverages Groq (Llama 3.3 70B) to deliver context-aware intelligence:
- **Purpose**: What the package does on *your* machine.
- **Context Inference**: Explains *why* you likely installed it based on your other installed packages (e.g., knowing you have `neovim` helps it infer why `lua51` exists).
- **Safety Verdict**: Clear `[KEEP]`, `[CAUTION]`, or `[SAFE]` verdict for removals.
- **Launch Command**: Extracts and displays terminal launch commands (`Command: ...`).
- **Deduplicated & Cached**: Singleflight concurrency deduplication and persistent caching ensure network requests are never repeated for cached packages.

###  Fuzzy Search & Package Installation (`i`)
- Press `i` to open the interactive **Install Modal**.
- Live fuzzy searching powered by multi-token matching, subsequence scoring, and repository weighting (`[core]` > `[extra]` > `[multilib]` > `[aur]`).
- Press `Tab` inside search results to view package descriptions and AI-generated quick summaries before installing.
- Direct installation for official repositories and AUR packages via `yay`/`paru` or `pacman`.

###  Full System Upgrade (`U`) & Package Update (`u`)
- **Full Upgrade (`U`)**: Press `U` in any panel (or `u`/`U` in the sidebar) to trigger a full system/manager upgrade (`yay -Syu` / `pacman -Syu` / `flatpak update -y`) with real-time log streaming.
- **Targeted Update (`u`)**: Press `u` on highlighted or multi-selected packages to update only those specific packages.

###  Orphan Package Cleanup (`o`)
- Press `o` to detect all orphaned dependencies (`pacman -Qtdq`).
- Batch remove all unused orphans in a single operation.

###  Yazi-Style Multi-Selection & Batch Removal (`x`)
- `Space`: Toggle individual package selection.
- `v`: Enter **Visual Mode** — select a contiguous range with `j`/`k`.
- `x`: Batch uninstall all selected packages in a single privileged `sudo` transaction.

###  Persistent Cache
Analysis results are saved locally to `~/.cache/orpheus/analysis.json` in clean, human-readable JSON. Analyze once, read instantly forever.

---

##  Keybindings

Orpheus uses intuitive **vim-style keybindings** throughout. The status bar at the bottom always displays context-sensitive shortcuts.

### Global & Navigation
| Key | Action |
|---|---|
| `j` / `k` / `↓` / `↑` | Move down / up |
| `h` / `l` / `←` / `→` | Focus left / right panel |
| `Ctrl+D` / `Ctrl+U` | Half-page down / up |
| `G` | Jump to bottom |
| `gg` | Jump to top |
| `Tab` | Switch active package manager (Pacman ↔ Flatpak) |
| `Enter` / `l` | Open detail / commit selection / confirm |
| `Esc` | Clear visual selection / cancel / exit search / back |
| `q` | Quit Orpheus |

### Selection & Filtering
| Key | Action |
|---|---|
| `Space` | Toggle package selection |
| `v` | Visual range selection mode |
| `/` | Live filter / search installed packages |
| `s` | Cycle sort: **Name → Size → Install Date** |
| `r` | Reload package list from active manager |

### Package Actions
| Key | Action |
|---|---|
| `a` | Trigger AI analysis for selected package |
| `x` | Remove highlighted or multi-selected packages |
| `i` | Open Package Search & Install modal (Official repos, AUR, Flatpak) |
| `u` | Update highlighted or selected package(s) |
| `U` | **Full Upgrade** for the active package manager (`yay -Syu` / `flatpak update`) |
| `o` | Orphan packages inspection and cleanup |

---

##  Installation

### Prerequisites

- **Go 1.26+**
- **Arch Linux** (or Arch-based distribution)
- **`yay`** or **`paru`** *(optional, recommended)*: For AUR package search and installation.
- **`flatpak`** *(optional)*: For Flatpak application management.

### Build from Source

```bash
git clone https://github.com/your-username/orpheus.git
cd orpheus
go build -o orpheus .
```

### Configuration

Create a `.env` file in the project directory (or in the same directory as the `orpheus` binary):

```env
GROQ_API_KEY=your_groq_api_key_here
# Optional custom model (defaults to llama-3.3-70b-versatile)
# ORPHEUS_MODEL=llama-3.3-70b-versatile
```

#### Get a Free Groq API Key:
1. Visit [console.groq.com](https://console.groq.com).
2. Create a free account and generate an API key.
3. Add the key to your `.env` file.

### Run

```bash
./orpheus
```

> **Note:** Orpheus automatically resolves `.env` relative to the executable binary location, so you can symlink or move the binary anywhere (e.g. `~/.local/bin/orpheus`).

---

##  Architecture

```
orpheus/
├── main.go                  # Entry point — loads .env, initializes Bubble Tea program
├── go.mod / go.sum          # Module dependencies
├── .env                     # API configuration (ignored in git)
│
└── internal/
    ├── ai/
    │   ├── analyzer.go      # Groq/Llama AI client, prompt builders, retry & rate limit protection
    │   └── analyzer_test.go # Deduplication and backoff tests
    │
    ├── cache/
    │   ├── cache.go         # Thread-safe JSON file cache (sync.RWMutex)
    │   └── cache_test.go    # Cache lookup and fallback tests
    │
    ├── pm/                  # Package Manager Abstraction Layer
    │   ├── package.go       # Package struct & Manager interface
    │   ├── pacman.go        # Pacman/AUR provider (-Qi parser, search, orphans, install, update)
    │   ├── flatpak.go       # Flatpak provider (list, search, install, update, cleanup)
    │   ├── fuzzy.go         # Fuzzy matching & repository relevance ranking engine
    │   └── fuzzy_test.go    # Fuzzy scoring test suite
    │
    └── tui/                 # Terminal UI (Bubble Tea + Lip Gloss)
        ├── model.go         # State machine, model definitions, async commands
        ├── msgs.go          # Message types
        ├── update.go        # Key handlers, modal state logic, execution loops
        ├── view.go          # Layout rendering (sidebar, list, detail, modals, status bar)
        └── styles.go        # Gruvbox Dark design system and Lip Gloss styles
```

---

---

##  Contributing

Contributions are welcome! Adding a new package manager is simple — implement the `pm.Manager` interface in `internal/pm/`:

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

---

##  License

MIT License. See [LICENSE](LICENSE) for details.

---

<div align="center">

**Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)  and [Lip Gloss](https://github.com/charmbracelet/lipgloss) **

*Orpheus — Package management made clean, fast, and intelligent.*

</div>
