package tui

import (
	"packichu/internal/pm"
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
	err    error
	output string
}

type pkgUpdateOutputMsg struct {
	err    error
	output string
}

type pkgInstallOutputMsg struct {
	chunk string
}

type installAIAnalysisMsg struct {
	pkgKey string
	text   string
	err    error
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

// aiSearchResultMsg is sent when semantic AI search finishes
type aiSearchResultMsg struct {
	PkgNames []string
	Err      error
}

type orphansCheckedMsg struct {
	orphans []string
	err     error
}

type allUpdatablesCheckedMsg struct {
	updatables map[string][]pm.UpdatablePackage
}

type aiUpdateAnalysisMsg struct {
	text string
	err  error
}

type cleanCacheOutputMsg struct {
	err    error
	output string
}
