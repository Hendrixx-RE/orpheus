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

func TestFlatpakManager(t *testing.T) {
	f := NewFlatpak()
	if f.Name() != "flatpak" {
		t.Errorf("expected Flatpak name 'flatpak', got '%s'", f.Name())
	}
	if f.RequiresSudo() {
		t.Errorf("expected Flatpak to not require sudo")
	}

	size := parseFlatpakSize("15.5 MB")
	if size <= 0 {
		t.Errorf("expected parsed size > 0, got %d", size)
	}
}

func TestParseDate(t *testing.T) {
	testDates := []string{
		"Fri 08 May 2026 04:56:09 PM IST",
		"Sun 16 Aug 2026 02:47:20 AM IST",
		"Mon 02 Jan 2006 15:04:05 MST",
		"2026-08-24 10:30:00",
	}
	for _, d := range testDates {
		parsed := parseDate(d)
		if parsed.IsZero() {
			t.Errorf("failed to parse valid date: %q", d)
		}
	}
}
