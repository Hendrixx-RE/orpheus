package tui

import (
	"orpheus/internal/pm"
	"orpheus/internal/svc"
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

type svcsLoadedMsg struct {
	svcs []svc.Service
	err  error
}

type orphansLoadedMsg struct {
	pkgs []pm.Package
	err  error
}
