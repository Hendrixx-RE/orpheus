# Orpheus — Todo List

## ✅ Done
- [x] Pacman provider
- [x] npm provider
- [x] Flatpak provider (with --delete-data + --unused cleanup)
- [x] AI analysis with Groq/OpenAI/Anthropic/Gemini
- [x] AI now returns terminal command to launch packages (Command: ...)
- [x] Cache formatted with MarshalIndent (ripgrep-ready)
- [x] Ripgrep AI Cache Search integration (`?` keybind)
- [x] Background batch analysis on startup
- [x] Orphan package cleanup keymap (`o` keybind) per provider

## 🔨 In Progress

## 📋 Planned

### Search & Install via yay (Priority: High)
- [ ] Background cacher: dump all available packages to `~/.cache/orpheus/available.txt`
  - `pacman -Sl` for official repos
  - `yay -Slq` for AUR packages
  - `flatpak remote-ls` for Flatpak apps
  - Runs silently on startup in a background goroutine
- [ ] Install mode UI: press `i` to enter instant search bar
  - Ripgrep filters `available.txt` in real-time as you type
  - Shows results from all managers unified
- [ ] Install execution: press Enter to install via `yay -S` (handles both official + AUR)
  - Flatpak installs via `flatpak install`

### New Package Manager Providers
- [ ] Rust (`cargo install` binaries)
- [ ] Go (`go install` binaries)
- [ ] pipx (isolated Python tools)

### Other Features
- [ ] Global search across all managers simultaneously
- [ ] Update command for each package manager
- [ ] Cache clear from the UI
- [ ] Leftover config finder — use ripgrep to scan ~/.config for orphaned configs after uninstall
- [ ] "Who owns this file?" — reverse lookup using rg on /var/lib/pacman/local/*/files
- [ ] Services monitoring (maybe)
