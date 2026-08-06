package tui

import (
	"orpheus/internal/pm"
)

type pkgsLoadedMsg struct {
	pkgs []pm.Package
	err  error
}

type pkgDetailMsg struct {
	pkg *pm.Package
	err error
}

type aiAnalysisMsg struct {
	text string
	err  error
}

type pkgRemovedMsg struct {
	err error
}

type pkgInstalledMsg struct {
	err error
}

// BatchProgressMsg is sent by your background goroutine to update the UI
type BatchProgressMsg struct {
	Total   int
	Current int
	PkgName string
	Done    bool
}

type processNextBatchPkgMsg struct {
	MissingPkgs   []pm.Package
	CurrentIdx    int
	ExplicitNames []string
}

// aiSearchResultMsg is sent when your ripgrep logic finishes
type aiSearchResultMsg struct {
	PkgNames []string
	Err      error
}

type orphansCheckedMsg struct {
	orphans []string
	err     error
}
