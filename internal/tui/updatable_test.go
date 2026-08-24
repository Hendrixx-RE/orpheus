package tui

import (
	"testing"
	"time"

	"packichu/internal/pm"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdatableFiltering(t *testing.T) {
	m := New()
	m.allPkgs = []pm.Package{
		{Name: "neovim", Version: "0.10.0", InstallReason: "Explicitly installed"},
		{Name: "git", Version: "2.40.0", InstallReason: "Explicitly installed"},
		{Name: "curl", Version: "8.0.0", InstallReason: "Explicitly installed"},
		{Name: "glibc", Version: "2.38", InstallReason: "Installed as a dependency"},
	}
	m.updatableMap = map[string]pm.UpdatablePackage{
		"neovim": {Name: "neovim", OldVersion: "0.10.0", NewVersion: "0.10.1", Manager: "pacman"},
		"curl":   {Name: "curl", OldVersion: "8.0.0", NewVersion: "8.1.0", Manager: "pacman"},
	}

	// Default: all explicit packages
	m.showUpdatableOnly = false
	m.applyFilter()
	if len(m.filteredPkgs) != 3 {
		t.Fatalf("expected 3 explicit packages, got %d", len(m.filteredPkgs))
	}

	// Filter updatable only (only explicit packages with updates are shown)
	m.updatableMap["glibc"] = pm.UpdatablePackage{Name: "glibc", OldVersion: "2.38", NewVersion: "2.39", Manager: "pacman"}
	m.showUpdatableOnly = true
	m.applyFilter()
	if len(m.filteredPkgs) != 2 {
		t.Fatalf("expected 2 explicit updatable packages (excluding glibc dependency), got %d", len(m.filteredPkgs))
	}

	expected := map[string]bool{"neovim": true, "curl": true}
	for _, p := range m.filteredPkgs {
		if !expected[p.Name] {
			t.Errorf("unexpected package in filtered results: %s", p.Name)
		}
	}
}

func TestUpdateModalFlowAndAIPreview(t *testing.T) {
	m := New()
	m.filteredPkgs = []pm.Package{
		{Name: "curl", Version: "8.0.0", InstallReason: "Explicitly installed"},
	}
	m.updatableMap = map[string]pm.UpdatablePackage{
		"curl": {Name: "curl", OldVersion: "8.0.0", NewVersion: "8.1.0", Manager: "pacman"},
	}
	m.selectedPkgs["curl"] = true

	m, _ = m.startUpdate()
	if !m.updatingModal {
		t.Fatalf("expected updatingModal to be true")
	}
	if m.updateAskPassword {
		t.Fatalf("expected updateAskPassword to be false initially (confirmation/AI options phase)")
	}

	// Press 'a' to ask AI
	m, _ = m.handleUpdateModalKey("a", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !m.updateShowAIPreview || !m.updateAILoading {
		t.Fatalf("expected updateShowAIPreview and updateAILoading to be true after pressing 'a'")
	}

	// Receive AI message
	aiMsg := aiUpdateAnalysisMsg{text: "Curl 8.1.0 changelog summary", err: nil}
	mUpdated, _ := m.Update(aiMsg)
	m = mUpdated.(Model)

	if m.updateAILoading {
		t.Fatalf("expected updateAILoading to be false after msg")
	}
	if m.updateAIText != "Curl 8.1.0 changelog summary" {
		t.Errorf("expected changelog text, got %q", m.updateAIText)
	}

	// Press 'Enter' to proceed to password/execution
	m, _ = m.handleUpdateModalKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	if !m.updateAskPassword && !m.updatingLoading {
		t.Fatalf("expected updateAskPassword or updatingLoading to be true after Enter")
	}
}

func TestCleanCacheModalFlow(t *testing.T) {
	m := New()
	m, _ = m.startCleanCache()
	if !m.cleanCacheModal {
		t.Fatalf("expected cleanCacheModal to be true")
	}

	// Press 'Enter' to confirm clean cache
	m, _ = m.handleCleanCacheModalKey("enter", tea.KeyMsg{Type: tea.KeyEnter})
	if !m.cleanCacheAskPassword && !m.cleanCacheLoading {
		t.Fatalf("expected cleanCacheAskPassword or cleanCacheLoading to be true after Enter")
	}

	// Test message receipt
	cleanMsg := cleanCacheOutputMsg{output: "Package cache cleaned."}
	mUpdated, _ := m.Update(cleanMsg)
	m = mUpdated.(Model)

	if !m.cleanCacheDone {
		t.Fatalf("expected cleanCacheDone to be true")
	}
	if m.cleanCacheOutput != "Package cache cleaned." {
		t.Errorf("expected clean cache output, got %q", m.cleanCacheOutput)
	}
}

func TestSortByDate(t *testing.T) {
	m := New()
	now := time.Now()
	m.allPkgs = []pm.Package{
		{Name: "pkg-old", Version: "1.0", InstallDate: now.Add(-10 * time.Hour), InstallReason: "Explicitly installed"},
		{Name: "pkg-new", Version: "2.0", InstallDate: now, InstallReason: "Explicitly installed"},
		{Name: "pkg-mid", Version: "1.5", InstallDate: now.Add(-5 * time.Hour), InstallReason: "Explicitly installed"},
	}

	m.sortMode = sortByDate
	m.applyFilter()

	if len(m.filteredPkgs) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(m.filteredPkgs))
	}
	if m.filteredPkgs[0].Name != "pkg-new" {
		t.Errorf("expected newest package first, got %s", m.filteredPkgs[0].Name)
	}
	if m.filteredPkgs[1].Name != "pkg-mid" {
		t.Errorf("expected mid package second, got %s", m.filteredPkgs[1].Name)
	}
	if m.filteredPkgs[2].Name != "pkg-old" {
		t.Errorf("expected oldest package third, got %s", m.filteredPkgs[2].Name)
	}
}
