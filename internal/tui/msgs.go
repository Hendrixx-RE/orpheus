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
