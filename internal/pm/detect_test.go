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

	// Since testing on Arch with pacman, pacman should always be detected
	if !names["pacman"] && !names["flatpak"] {
		t.Errorf("expected at least pacman or flatpak detected, got: %v", names)
	}
}
