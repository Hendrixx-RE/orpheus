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
	pkgKey string
	text   string
	err    error
}

type pkgRemovedMsg struct {
	err error
}

type pkgInstalledMsg struct {
	err error
}

type pkgSearchResultMsg struct {
	results []pm.Package
	err     error
}

// BatchProgressMsg is sent by your background goroutine to update the UI
type BatchProgressMsg struct {
	BatchID uint64
	Total   int
	Current int
	PkgName string
	Done    bool
}

type processNextBatchPkgMsg struct {
	BatchID       uint64
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
