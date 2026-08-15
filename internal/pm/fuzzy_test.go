package pm

import (
	"testing"
)

func TestFuzzyScoreRanking(t *testing.T) {
	pkgs := []Package{
		{Name: "python-neovim", Description: "[extra] Python client for Neovim"},
		{Name: "neovim-qt", Description: "[extra] Qt GUI for Neovim"},
		{Name: "neovim", Description: "[extra] Vim-fork focused on extensibility and usability"},
		{Name: "neovim-remote", Description: "[extra] Support for controlling Neovim processes"},
		{Name: "aur-neovim-plugin", Description: "[aur] Neovim plugin"},
	}

	ranked := RankSearchResults("neovim", pkgs)

	if len(ranked) != len(pkgs) {
		t.Fatalf("expected %d packages, got %d", len(pkgs), len(ranked))
	}

	// Exact match "neovim" MUST be ranked #1 at the top
	if ranked[0].Name != "neovim" {
		t.Errorf("expected top result to be 'neovim', got '%s'", ranked[0].Name)
	}

	// "neovim-qt" (prefix match) should rank higher than "python-neovim" (substring match)
	if ranked[1].Name != "neovim-qt" && ranked[1].Name != "neovim-remote" {
		t.Errorf("expected 2nd result to be a prefix match (neovim-qt/neovim-remote), got '%s'", ranked[1].Name)
	}
}

func TestSubsequenceFuzzyMatching(t *testing.T) {
	pkgs := []Package{
		{Name: "ripgrep", Description: "[extra] Line-oriented search tool"},
		{Name: "python-rg", Description: "[aur] Python package"},
		{Name: "liberation-fonts", Description: "[extra] Fonts"},
	}

	ranked := RankSearchResults("rg", pkgs)

	if len(ranked) == 0 {
		t.Fatalf("expected results for 'rg', got 0")
	}

	// "python-rg" (exact word) or "ripgrep" (fuzzy subsequence match) must be ranked above liberation-fonts
	if ranked[0].Name != "python-rg" && ranked[0].Name != "ripgrep" {
		t.Errorf("expected top match for 'rg' to be python-rg or ripgrep, got '%s'", ranked[0].Name)
	}
}

func TestMultiTokenSearch(t *testing.T) {
	pkgs := []Package{
		{Name: "code-oss", Description: "[extra] Open-source Code editor"},
		{Name: "visual-studio-code-bin", Description: "[aur] Visual Studio Code binary"},
		{Name: "kcodecs", Description: "[extra] KDE codecs"},
	}

	ranked := RankSearchResults("visual studio code", pkgs)

	if len(ranked) == 0 {
		t.Fatalf("expected results for 'visual studio code'")
	}

	if ranked[0].Name != "visual-studio-code-bin" {
		t.Errorf("expected 'visual-studio-code-bin' at top, got '%s'", ranked[0].Name)
	}
}
