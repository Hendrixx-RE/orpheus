package pm

import (
	"os/exec"
)

// DetectManagers inspects the host system for available package manager binaries
// and returns all detected Manager implementations (Pacman, AUR, Flatpak).
func DetectManagers() []Manager {
	var managers []Manager

	// 1. Native Pacman (Official Repositories)
	if _, err := exec.LookPath("pacman"); err == nil {
		managers = append(managers, NewPacman())
	}

	// 2. Arch User Repository (via yay or paru helper)
	if _, err := exec.LookPath("yay"); err == nil {
		managers = append(managers, NewAUR("yay"))
	} else if _, err := exec.LookPath("paru"); err == nil {
		managers = append(managers, NewAUR("paru"))
	}

	// 3. Flatpak Sandboxed Applications
	if _, err := exec.LookPath("flatpak"); err == nil {
		managers = append(managers, NewFlatpak())
	}

	// Fallback to Pacman if no manager was detected
	if len(managers) == 0 {
		managers = append(managers, NewPacman())
	}

	return managers
}
