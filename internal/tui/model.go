// Package tui handles the tui
package tui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"orpheus/internal/ai"
	"orpheus/internal/cache"
	"orpheus/internal/pm"
	"orpheus/internal/svc"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewID int

const (
	viewPackages viewID = iota
	viewCleanup
	viewServices
	viewWhitelist
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

type filterReason int

const (
	filterAll filterReason = iota
	filterExplicit
	filterDeps
)

type Model struct {
	width  int
	height int
	ready  bool

	activeView   viewID
	focusedPanel panelID

	// packages
	allPkgs             []pm.Package
	filteredPkgs        []pm.Package
	listCursor          int
	listOffset          int
	loading             bool
	sortMode            sortMode
	hideSystem          bool
	filterReason        filterReason
	whitelist           map[string]bool
	excludedByWhitelist map[string]bool

	// search
	searching   bool
	searchInput textinput.Model

	// selected package detail
	selectedPkg *pm.Package
	detailVP    viewport.Model
	aiText      string
	aiLoading   bool
	aiErr       string

	// cleanup view
	cleanupPkgs   []pm.Package
	cleanupCursor int
	cleanupLoaded bool

	// services
	services  []svc.Service
	svcCursor int
	svcLoaded bool

	spinner spinner.Model
	lastKey string

	err error

	mgr   pm.Manager
	aiSvc *ai.Analyzer
	cache *cache.Cache
}

func New() Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorPurple)

	ti := textinput.New()
	ti.Placeholder = "search packages..."
	ti.CharLimit = 64

	vp := viewport.New(0, 0)

	c, _ := cache.New()

	m := Model{
		spinner:     sp,
		searchInput: ti,
		detailVP:    vp,
		loading:     true,
		mgr:         pm.NewPacman(),
		aiSvc:       ai.New(),
		cache:       c,
		whitelist:   make(map[string]bool),
	}
	m.loadWhitelist()
	return m
}

func (m *Model) loadWhitelist() {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	path := filepath.Join(dir, "orpheus", "whitelist.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.whitelist)
}

func (m *Model) saveWhitelist() {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	path := filepath.Join(dir, "orpheus", "whitelist.json")
	data, _ := json.Marshal(m.whitelist)
	os.WriteFile(path, data, 0o644)
}

func (m *Model) computeExcluded() {
	m.excludedByWhitelist = make(map[string]bool)
	if len(m.whitelist) == 0 {
		return
	}

	// Build dependency map
	depsMap := make(map[string][]string)
	for _, p := range m.allPkgs {
		depsMap[p.Name] = p.Dependencies
	}

	var visit func(string)
	visit = func(name string) {
		if m.excludedByWhitelist[name] {
			return
		}
		m.excludedByWhitelist[name] = true
		for _, d := range depsMap[name] {
			visit(d)
		}
	}

	for name := range m.whitelist {
		visit(name)
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		loadPackages(m.mgr),
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

func analyzePackage(a *ai.Analyzer, c *cache.Cache, pkg *pm.Package) tea.Cmd {
	return func() tea.Msg {
		key := pkg.Name + "@" + pkg.Version
		if text, ok := c.Get(key); ok {
			return aiAnalysisMsg{text: text}
		}
		text, err := a.Analyze(context.Background(), pkg)
		if err != nil {
			return aiAnalysisMsg{err: err}
		}
		c.Set(key, text)
		return aiAnalysisMsg{text: text}
	}
}

func loadServices() tea.Cmd {
	return func() tea.Msg {
		svcs, err := svc.ListServices()
		if err != nil {
			return svcsLoadedMsg{err: err}
		}
		return svcsLoadedMsg{svcs: svcs}
	}
}

func loadOrphans() tea.Cmd {
	return func() tea.Msg {
		pkgs, err := pm.GetOrphansDetailed()
		if err != nil {
			return orphansLoadedMsg{err: err}
		}
		sort.Slice(pkgs, func(i, j int) bool {
			return pkgs[i].Size > pkgs[j].Size
		})
		return orphansLoadedMsg{pkgs: pkgs}
	}
}

// helpers

func (m *Model) applyFilter() {
	q := m.searchInput.Value()
	var out []pm.Package

	for _, p := range m.allPkgs {
		if m.activeView != viewWhitelist && m.excludedByWhitelist[p.Name] {
			continue
		}
		if m.hideSystem && p.IsSystem {
			continue
		}
		if m.filterReason == filterExplicit && p.InstallReason != "Explicitly installed" {
			continue
		}
		if m.filterReason == filterDeps && p.InstallReason == "Explicitly installed" {
			continue
		}
		if q == "" || contains(p.Name, q) || contains(p.Description, q) {
			out = append(out, p)
		}
	}

	if m.activeView == viewWhitelist {
		out = make([]pm.Package, 0)
		for _, p := range m.allPkgs {
			if m.whitelist[p.Name] {
				out = append(out, p)
			}
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
	if m.activeView == viewCleanup {
		return m.cleanupPkgs
	}
	return m.filteredPkgs
}

func (m *Model) currentCursor() int {
	if m.activeView == viewCleanup {
		return m.cleanupCursor
	}
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
	if m.activeView == viewCleanup {
		m.cleanupCursor = clamp(m.cleanupCursor+delta, 0, n-1)
	} else {
		m.listCursor = clamp(m.listCursor+delta, 0, n-1)
		m.ensureVisible()
	}
}

func (m *Model) jumpTop() {
	if m.activeView == viewCleanup {
		m.cleanupCursor = 0
	} else {
		m.listCursor = 0
		m.listOffset = 0
	}
}

func (m *Model) jumpBottom() {
	n := m.listLen()
	if n == 0 {
		return
	}
	if m.activeView == viewCleanup {
		m.cleanupCursor = n - 1
	} else {
		m.listCursor = n - 1
		m.ensureVisible()
	}
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
	return m.height - 4 // borders + status bar + header
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
