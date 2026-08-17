package pm

import (
	"os/exec"
)

// DetectManagers inspects the host system for available package manager binaries
// and returns all detected Manager implementations.
func DetectManagers() []Manager {
	var managers []Manager

	if _, err := exec.LookPath("pacman"); err == nil {
		managers = append(managers, NewPacman())
	}
	if _, err := exec.LookPath("flatpak"); err == nil {
		managers = append(managers, NewFlatpak())
	}

	// Fallback to Pacman if no manager was detected
	if len(managers) == 0 {
		managers = append(managers, NewPacman())
	}

	return managers
}
