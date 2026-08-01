// Package tui handles the tui
package tui

import (
	"context"
	"sort"

	"orpheus/internal/ai"
	"orpheus/internal/cache"
	"orpheus/internal/pm"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type panelID int

const (
	panelSidebar panelID = iota
	panelList
	panelDetail
)

type sortMode int

const (
	sortByName sortMode = iota
	sortBySize
	sortByDate
)

type Model struct {
	width  int
	height int
	ready  bool

	focusedPanel panelID

	// packages
	allPkgs      []pm.Package
	filteredPkgs []pm.Package
	listCursor   int
	listOffset   int
	loading      bool
	sortMode     sortMode

	// search
	searching   bool
	searchInput textinput.Model

	// ai search
	aiSearching   bool
	aiSearchInput textinput.Model

	// batch background sync
	batchActive  bool
	batchTotal   int
	batchCurrent int
	batchPkg     string
	progress     progress.Model

	// selected package detail
	selectedPkg *pm.Package
	detailVP    viewport.Model
	aiText      string
	aiLoading   bool
	aiErr       string

	// removal state
	askingPassword  bool
	removingLoading bool
	removeErr       string
	passwordInput   textinput.Model

	spinner spinner.Model
	lastKey string

	err error

	selectedPkgs map[string]bool
	visualMode   bool
	visualStart  int

	managers  []pm.Manager
	activeMgr int
	aiSvc     *ai.Analyzer
	cache     *cache.Cache
}

func New() Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorPurple)

	ti := textinput.New()
	ti.Placeholder = "search packages..."
	ti.CharLimit = 64

	pi := textinput.New()
	pi.Placeholder = "password"
	pi.EchoMode = textinput.EchoPassword
	pi.EchoCharacter = '•'
	pi.CharLimit = 64

	aiTi := textinput.New()
	aiTi.Placeholder = "ripgrep ai cache..."
	aiTi.CharLimit = 64

	vp := viewport.New(0, 0)

	prg := progress.New(progress.WithDefaultGradient())

	c, _ := cache.New()

	return Model{
		spinner:       sp,
		searchInput:   ti,
		aiSearchInput: aiTi,
		passwordInput: pi,
		progress:      prg,
		detailVP:      vp,
		loading:       true,
		selectedPkgs:  make(map[string]bool),
		managers:      []pm.Manager{pm.NewPacman(), pm.NewFlatpak(), pm.NewNpm()},
		activeMgr:     0,
		aiSvc:         ai.New(),
		cache:         c,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		loadPackages(m.managers[m.activeMgr]),
	)
}

// commands

func loadPackages(mgr pm.Manager) tea.Cmd {
	return func() tea.Msg {
		pkgs, err := mgr.ListAll()
		return pkgsLoadedMsg{pkgs: pkgs, err: err}
	}
}

func loadPackageDetail(mgr pm.Manager, name string) tea.Cmd {
	return func() tea.Msg {
		pkg, err := mgr.GetPackage(name)
		return pkgDetailMsg{pkg: pkg, err: err}
	}
}

func analyzePackage(a *ai.Analyzer, c *cache.Cache, pkg *pm.Package, explicitNames []string) tea.Cmd {
	return func() tea.Msg {
		key := pkg.Name + "@" + pkg.Version
		if text, ok := c.Get(key); ok {
			return aiAnalysisMsg{text: text}
		}
		text, err := a.Analyze(context.Background(), pkg, explicitNames)
		if err != nil {
			return aiAnalysisMsg{err: err}
		}
		c.Set(key, text)
		return aiAnalysisMsg{text: text}
	}
}

// helpers

func (m *Model) applyFilter() {
	q := m.searchInput.Value()
	var out []pm.Package

	for _, p := range m.allPkgs {
		if p.InstallReason != "Explicitly installed" {
			continue
		}
		if q == "" || contains(p.Name, q) || contains(p.Description, q) {
			out = append(out, p)
		}
	}

	switch m.sortMode {
	case sortBySize:
		sort.Slice(out, func(i, j int) bool {
			return out[i].Size > out[j].Size
		})
	case sortByDate:
		sort.Slice(out, func(i, j int) bool {
			return out[i].InstallDate.After(out[j].InstallDate)
		})
	case sortByName:
		sort.Slice(out, func(i, j int) bool {
			return out[i].Name < out[j].Name
		})
	}

	m.filteredPkgs = out
	if m.listCursor >= len(m.filteredPkgs) {
		m.listCursor = max(0, len(m.filteredPkgs)-1)
	}
}

func (m *Model) currentList() []pm.Package {
	return m.filteredPkgs
}

func (m *Model) isPkgSelected(i int) bool {
	if i < 0 || i >= len(m.filteredPkgs) {
		return false
	}
	name := m.filteredPkgs[i].Name
	if m.selectedPkgs[name] {
		return true
	}
	if m.visualMode {
		minIdx := minI(m.visualStart, m.listCursor)
		maxIdx := maxI(m.visualStart, m.listCursor)
		if i >= minIdx && i <= maxIdx {
			return true
		}
	}
	return false
}

func (m *Model) commitVisualSelection() {
	if !m.visualMode {
		return
	}
	minIdx := minI(m.visualStart, m.listCursor)
	maxIdx := maxI(m.visualStart, m.listCursor)
	for i := minIdx; i <= maxIdx; i++ {
		m.selectedPkgs[m.filteredPkgs[i].Name] = true
	}
	m.visualMode = false
}

func (m *Model) getSelectedNames() []string {
	var names []string
	if len(m.selectedPkgs) > 0 {
		for k, v := range m.selectedPkgs {
			if v {
				names = append(names, k)
			}
		}
	} else if len(m.filteredPkgs) > 0 {
		names = append(names, m.filteredPkgs[m.listCursor].Name)
	}
	return names
}

func (m *Model) currentCursor() int {
	return m.listCursor
}

func (m *Model) listLen() int {
	return len(m.currentList())
}

func (m *Model) moveCursor(delta int) {
	n := m.listLen()
	if n == 0 {
		return
	}
	m.listCursor = clamp(m.listCursor+delta, 0, n-1)
	m.ensureVisible()
}

func (m *Model) jumpTop() {
	m.listCursor = 0
	m.listOffset = 0
}

func (m *Model) jumpBottom() {
	n := m.listLen()
	if n == 0 {
		return
	}
	m.listCursor = n - 1
	m.ensureVisible()
}

func (m *Model) ensureVisible() {
	h := m.listPanelHeight()
	if h <= 0 {
		return
	}
	if m.listCursor < m.listOffset {
		m.listOffset = m.listCursor
	}
	if m.listCursor >= m.listOffset+h {
		m.listOffset = m.listCursor - h + 1
	}
}

func (m Model) listPanelHeight() int {
	return m.height - 7 // panel height (m.height - 3) - borders (2) - title (1) - divider (1)
}

func (m Model) sidebarWidth() int { return 18 }
func (m Model) detailWidth() int  { return 46 }
func (m Model) listWidth() int {
	return m.width - m.sidebarWidth() - m.detailWidth() - 6
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func contains(s, sub string) bool {
	if sub == "" {
		return true
	}
	return len(s) >= len(sub) && indexCI(s, sub) >= 0
}

func indexCI(s, sub string) int {
	sl := toLower(s)
	subl := toLower(sub)
	for i := 0; i <= len(sl)-len(subl); i++ {
		if sl[i:i+len(subl)] == subl {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}
