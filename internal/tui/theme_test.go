package tui

import (
	"testing"
)

func TestThemeCycling(t *testing.T) {
	if len(Themes) != 3 {
		t.Fatalf("expected 3 themes, got %d", len(Themes))
	}

	expectedNames := []string{"Gruvbox Retro", "Catppuccin", "Monokai"}
	for i, name := range expectedNames {
		if Themes[i].Name != name {
			t.Errorf("expected theme %d to be %q, got %q", i, name, Themes[i].Name)
		}
	}

	m := New()
	if m.themeIdx != 0 {
		t.Errorf("expected initial themeIdx 0, got %d", m.themeIdx)
	}

	m.CycleTheme()
	if m.themeIdx != 1 {
		t.Errorf("expected themeIdx 1 after 1st cycle, got %d", m.themeIdx)
	}
	if Themes[m.themeIdx].Name != "Catppuccin" {
		t.Errorf("expected Catppuccin theme, got %s", Themes[m.themeIdx].Name)
	}

	m.CycleTheme()
	if m.themeIdx != 2 {
		t.Errorf("expected themeIdx 2 after 2nd cycle, got %d", m.themeIdx)
	}
	if Themes[m.themeIdx].Name != "Monokai" {
		t.Errorf("expected Monokai theme, got %s", Themes[m.themeIdx].Name)
	}

	m.CycleTheme()
	if m.themeIdx != 0 {
		t.Errorf("expected themeIdx 0 after 3rd cycle, got %d", m.themeIdx)
	}
	if Themes[m.themeIdx].Name != "Gruvbox Retro" {
		t.Errorf("expected Gruvbox Retro theme, got %s", Themes[m.themeIdx].Name)
	}
}
