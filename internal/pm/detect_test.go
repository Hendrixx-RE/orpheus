package pm

import (
	"testing"
)

func TestDetectManagers(t *testing.T) {
	mgrs := DetectManagers()
	if len(mgrs) == 0 {
		t.Fatalf("expected at least one package manager detected or fallback, got 0")
	}

	names := make(map[string]bool)
	for _, m := range mgrs {
		if m == nil {
			t.Fatalf("detected nil manager")
		}
		if m.Name() == "" {
			t.Fatalf("manager has empty name")
		}
		names[m.Name()] = true
	}

	// Should detect pacman on Arch
	if !names["pacman"] && !names["aur"] && !names["flatpak"] {
		t.Errorf("expected at least pacman, aur, or flatpak detected, got: %v", names)
	}
}

func TestAURManagerCreation(t *testing.T) {
	aur := NewAUR("")
	if aur.Name() != "aur" {
		t.Errorf("expected AUR name 'aur', got '%s'", aur.Name())
	}
	if !aur.RequiresSudo() {
		t.Errorf("expected AUR to require sudo")
	}
	installCmd := aur.InstallCmd("my-pkg")
	if len(installCmd) < 4 || installCmd[len(installCmd)-1] != "my-pkg" {
		t.Errorf("unexpected install command: %v", installCmd)
	}
}
