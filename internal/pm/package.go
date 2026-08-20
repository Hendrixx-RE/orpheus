// Package pm provides common structures and interfaces for package management.
package pm

import (
	"fmt"
	"time"
)

type Package struct {
	Name          string
	Version       string
	Description   string
	Architecture  string
	Size          int64
	InstallDate   time.Time
	BuildDate     time.Time
	InstallReason string
	Dependencies  []string
	OptDeps       []string
	OptFor        []string
	HasService    bool
	ServiceName   string
	ServiceStatus string
	IsSystem      bool
	Repository    string
	IsInstalled   bool
}

type UpdatablePackage struct {
	Name       string
	OldVersion string
	NewVersion string
	Manager    string
}

type Manager interface {
	Name() string
	ListAll() ([]Package, error)
	GetPackage(name string) (*Package, error)
	UninstallCmd(names []string) []string
	UninstallOrphansCmd() []string
	GetOrphans() ([]string, error)
	InstallCmd(name string) []string
	UpdateCmd() []string
	UpdatePackagesCmd(names []string) []string
	GetUpdatable() ([]UpdatablePackage, error)
	CleanCacheCmd() []string
	Search(query string) ([]Package, error)
	// RequiresSudo reports whether install/uninstall/update commands must be run
	// through sudo. Pacman always does; Flatpak manages its own
	// privilege escalation and must NOT be wrapped in sudo.
	RequiresSudo() bool
}

func (p *Package) SizeMB() float64 {
	return float64(p.Size) / (1024 * 1024)
}

func (p *Package) FormatSize() string {
	b := p.Size
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GiB", float64(b)/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.2f MiB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.2f KiB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
