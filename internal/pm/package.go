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
	RequiredBy    []string
	OptFor        []string
	IsOrphan      bool
	HasService    bool
	ServiceName   string
	ServiceStatus string
}

type Manager interface {
	Name() string
	ListAll() ([]Package, error)
	GetPackage(name string) (*Package, error)
}

func (p *Package) ReasonShort() string {
	if p.IsOrphan {
		return "ORPHAN"
	}
	if p.InstallReason == "Explicitly installed" {
		return "EXPLICIT"
	}
	return "DEP"
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
