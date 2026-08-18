
<div align="center">

# Pacseer

**A terminal-based package management dashboard for Arch Linux**

*Browse, inspect, AI-analyze, search, install, update/batch update, and uninstall/batch uninstall packages across multiple package managers — all from one unified TUI.*

![Pacseer Dashboard](https://github.com/user-attachments/assets/0837cbbd-aee3-43c3-b7b2-ca40ec37db53)

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Bubble Tea](https://img.shields.io/badge/Bubble_Tea-TUI-FF75B5?style=flat-square)](https://github.com/charmbracelet/bubbletea)
[![License](https://img.shields.io/badge/License-MIT-fabd2f?style=flat-square)](#license)

</div>

---

##  Why Pacseer?

Your system accumulates packages over time. Some you installed months ago and forgot about. Some are 500 MB chonks you used once. Some are mysterious dependencies you're not even sure what they do anymore.

**Pacseer gives you total clarity and control.** It pulls every explicitly installed package from Pacman/AUR and Flatpak into a single dashboard, lets an AI analyze safety and system context, and provides one-key workflows for installing, updating, orphan cleaning, and batch uninstallation.

---

##  Features

###  Three-Panel Dashboard
A clean, vim-navigable interface:
- **Sidebar (Left)**: Switch between package managers (Pacman, Flatpak) and view high-level counts.
- **Package List (Middle)**: Real-time filtering, virtual scrolling, selection badges, and size formatting.
- **Detail Panel (Right)**: Comprehensive package metadata, dependencies, action status, and AI insights.

###  Multi-Manager Operations (Auto-Detected)
Pacseer inspects your machine and automatically activates only the package managers present on your system:
| Manager | What It Lists | Install / Search | Full Upgrade (`U`) | Uninstall Strategy |
|---|---|---|---|---|
| **Pacman** | Official Arch packages (`pacman -Qin`) | Official Repos (`[core]`, `[extra]`, `[multilib]`) | `pacman -Syu --noconfirm` | `pacman -Rns --noconfirm` (recursive dependency removal) |
| **AUR** | Foreign/AUR packages (`pacman -Qim`) | Arch User Repository (`yay -Ssa` / `paru -Ssa`) | `yay -Sua --noconfirm` / `paru -Sua` | `pacman -Rns --noconfirm` |
| **Flatpak** | Installed Flatpak desktop applications | Configured Flatpak remotes (Flathub, etc.) | `flatpak update -y` | Uninstalls app + deletes data + cleans unused runtimes |

###  AI-Powered Package Analysis
Press `a` on any package and Pacseer leverages Groq (Llama 3.3 70B) to deliver context-aware intelligence:
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
Analysis results are saved locally to `~/.cache/pacseer/analysis.json` in clean, human-readable JSON. Analyze once, read instantly forever.

---

##  Keybindings

Pacseer uses intuitive **vim-style keybindings** throughout. The status bar at the bottom always displays context-sensitive shortcuts.

### Global & Navigation
| Key | Action |
|---|---|
| `j` / `k` / `↓` / `↑` | Move down / up (details auto-display for hovered package) |
| `h` / `l` / `←` / `→` | Switch panel focus (Sidebar ↔ List ↔ Detail scroll) |
| `Ctrl+D` / `Ctrl+U` | Half-page down / up |
| `G` | Jump to bottom |
| `gg` | Jump to top |
| `Tab` | Switch active package manager (Pacman ↔ AUR ↔ Flatpak) |
| `l` / `Enter` | Focus detail panel (scroll view) / confirm modal |
| `Esc` | Clear visual selection / cancel / exit search / back |
| `q` | Quit Pacseer |

### Selection & Filtering
| Key | Action |
|---|---|
| `Space` | Toggle package selection |
| `v` | Visual range selection mode |
| `/` | Live filter / search installed packages |
| `s` | Cycle sort: **Name → Size → Install Date** |
| `t` | Cycle theme: **Gruvbox Retro → Catppuccin → Monokai** |
| `r` | Reload package list from active manager |

### Package Actions
| Key | Action |
|---|---|
| `a` | Trigger AI analysis for selected package |
| `x` | Remove highlighted or multi-selected packages |
| `i` | Open Package Search & Install modal (Official repos, AUR, Flatpak) |
| `u` | Update highlighted or selected package(s) |
| `U` | **Full System Upgrade** across all detected package managers (Pacman + AUR + Flatpak) |
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
git clone https://github.com/your-username/pacseer.git
cd pacseer
go build -o pacseer .
```

### Configuration

Configure your AI keys and model in `~/.config/pacseer/config.env` (or `~/.config/pacseer/.env`):

```bash
mkdir -p ~/.config/pacseer
```

Create `~/.config/pacseer/config.env`:

```env
# Pacseer auto-detects whichever provider key is present!
# Option 1: Google Gemini (Recommended - Free tier at https://aistudio.google.com)
GEMINI_API_KEY=AIzaSy...your_key_here

# Option 2: Groq (Free tier at https://console.groq.com)
# GROQ_API_KEY=your_groq_key_here

# Optional: Override the model (defaults to gemini-2.5-flash for Gemini, openai/gpt-oss-120b for Groq)
# PACSEER_MODEL=gemini-2.5-flash
```

---

##  Architecture

```
pacseer/
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

*Pacseer — Package management made clean, fast, and intelligent.*

</div>
