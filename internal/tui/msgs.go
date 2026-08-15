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

// syncProgressMsg is sent by the background sync worker to update the UI status
type syncProgressMsg struct {
	Total   int
	Done    int
	PkgName string
	DoneAll bool
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
