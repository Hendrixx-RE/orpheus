package tui

import (
	"strings"

	"orpheus/internal/pm"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.detailVP = viewport.New(m.detailWidth()-6, m.height-6)
		if m.selectedPkg != nil {
			m.detailVP.SetContent(m.buildDetailContent())
		}
		m.ready = true
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		// re-render detail content to animate the spinner while AI is loading
		if m.aiLoading && m.selectedPkg != nil {
			m.detailVP.SetContent(m.buildDetailContent())
		}
		return m, cmd

	case pkgsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.allPkgs = msg.pkgs
		m.filteredPkgs = msg.pkgs
		m.applyFilter()
		return m, nil

	case pkgDetailMsg:
		if msg.err == nil && msg.pkg != nil {
			m.selectedPkg = msg.pkg
			m.detailVP.SetContent(m.buildDetailContent())
			m.detailVP.GotoTop()
		}
		return m, nil

	case aiAnalysisMsg:
		m.aiLoading = false
		if msg.err != nil {
			m.aiErr = "AI unavailable: " + msg.err.Error()
		} else {
			m.aiText = msg.text
		}
		if m.selectedPkg != nil {
			m.detailVP.SetContent(m.buildDetailContent())
		}
		return m, nil

	case svcsLoadedMsg:
		m.svcLoaded = true
		if msg.err == nil {
			m.services = msg.svcs
		}
		return m, nil

	case orphansLoadedMsg:
		m.cleanupLoaded = true
		if msg.err == nil {
			m.cleanupPkgs = msg.pkgs
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if key == "q" && !m.searching {
			return m, tea.Quit
		}

		if m.searching {
			switch key {
			case "esc":
				m.searching = false
				m.searchInput.Blur()
				m.searchInput.SetValue("")
				m.applyFilter()
				return m, nil
			case "enter":
				m.searching = false
				m.searchInput.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.applyFilter()
				m.listCursor = 0
				m.listOffset = 0
				return m, cmd
			}
		}

		nm, cmd := m.handleKey(key)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return nm, tea.Batch(cmds...)
	}

	// Forward non-key messages to viewport when detail is focused
	if m.focusedPanel == panelDetail && m.selectedPkg != nil {
		var cmd tea.Cmd
		m.detailVP, cmd = m.detailVP.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleKey(key string) (Model, tea.Cmd) {
	// view switching
	switch key {
	case "1":
		m.activeView = viewPackages
		m.focusedPanel = panelList
		return m, nil
	case "2":
		m.activeView = viewCleanup
		m.focusedPanel = panelList
		if !m.cleanupLoaded {
			return m, loadOrphans()
		}
		return m, nil
	case "3":
		m.activeView = viewServices
		m.focusedPanel = panelList
		if !m.svcLoaded {
			return m, loadServices()
		}
		return m, nil
	case "tab":
		m.activeView = (m.activeView + 1) % 3
		m.focusedPanel = panelList
		if m.activeView == viewCleanup && !m.cleanupLoaded {
			return m, loadOrphans()
		}
		if m.activeView == viewServices && !m.svcLoaded {
			return m, loadServices()
		}
		return m, nil
	}

	// panel navigation
	switch key {
	case "h", "left":
		if m.focusedPanel > panelSidebar {
			m.focusedPanel--
		}
		m.lastKey = ""
		return m, nil
	}

	// detail panel
	if m.focusedPanel == panelDetail {
		return m.handleDetailKey(key)
	}

	// sidebar panel
	if m.focusedPanel == panelSidebar {
		return m.handleSidebarKey(key)
	}

	// list panel
	return m.handleListKey(key)
}

func (m Model) handleDetailKey(key string) (Model, tea.Cmd) {
	switch key {
	case "a":
		if m.selectedPkg != nil && !m.aiLoading {
			m.aiLoading = true
			m.aiText = ""
			m.aiErr = ""
			m.detailVP.SetContent(m.buildDetailContent())
			m.lastKey = ""
			return m, analyzePackage(m.aiSvc, m.cache, m.selectedPkg)
		}
	case "j", "down":
		m.detailVP.LineDown(1)
	case "k", "up":
		m.detailVP.LineUp(1)
	case "ctrl+d":
		m.detailVP.HalfViewDown()
	case "ctrl+u":
		m.detailVP.HalfViewUp()
	case "G":
		m.detailVP.GotoBottom()
		m.lastKey = ""
	case "g":
		if m.lastKey == "g" {
			m.detailVP.GotoTop()
			m.lastKey = ""
		} else {
			m.lastKey = "g"
		}
		return m, nil
	case "esc", "h":
		m.focusedPanel = panelList
	}
	m.lastKey = ""
	return m, nil
}

func (m Model) handleSidebarKey(key string) (Model, tea.Cmd) {
	switch key {
	case "j", "down":
		m.activeView = (m.activeView + 1) % 3
	case "k", "up":
		if m.activeView == 0 {
			m.activeView = 2
		} else {
			m.activeView--
		}
	case "l", "right", "enter":
		m.focusedPanel = panelList
	}
	m.lastKey = ""
	return m, nil
}

func (m Model) handleListKey(key string) (Model, tea.Cmd) {
	switch key {
	case "j", "down":
		m.moveCursor(1)
		m.lastKey = ""
	case "k", "up":
		m.moveCursor(-1)
		m.lastKey = ""
	case "ctrl+d":
		m.moveCursor(m.listPanelHeight() / 2)
		m.lastKey = ""
	case "ctrl+u":
		m.moveCursor(-m.listPanelHeight() / 2)
		m.lastKey = ""
	case "G":
		m.jumpBottom()
		m.lastKey = ""
	case "g":
		if m.lastKey == "g" {
			m.jumpTop()
			m.lastKey = ""
		} else {
			m.lastKey = "g"
		}
		return m, nil
	case "/":
		m.searching = true
		m.searchInput.Focus()
		m.lastKey = ""
	case "esc":
		if m.searchInput.Value() != "" {
			m.searchInput.SetValue("")
			m.applyFilter()
		}
		m.lastKey = ""
	case "enter", "l", "right":
		cmd := m.triggerSelect()
		m.lastKey = ""
		return m, cmd
	case "r":
		m.loading = true
		m.lastKey = ""
		return m, loadPackages(m.mgr)
	default:
		m.lastKey = ""
	}
	return m, nil
}

func (m *Model) triggerSelect() tea.Cmd {
	list := m.currentList()
	cursor := m.currentCursor()
	if cursor >= len(list) {
		return nil
	}
	pkg := list[cursor]
	m.focusedPanel = panelDetail

	// If already selected same package, just focus detail
	if m.selectedPkg != nil && m.selectedPkg.Name == pkg.Name && m.selectedPkg.Size > 0 {
		m.detailVP.GotoTop()
		return nil
	}

	// Show partial info immediately
	m.selectedPkg = &pkg
	m.aiText = ""
	m.aiErr = ""
	m.aiLoading = false
	m.detailVP.SetContent(m.buildDetailContent())
	m.detailVP.GotoTop()

	if pkg.Size == 0 {
		return loadPackageDetail(m.mgr, pkg.Name)
	}
	return nil
}

func (m Model) buildDetailContent() string {
	if m.selectedPkg == nil {
		return ""
	}
	p := m.selectedPkg
	var sb strings.Builder

	write := func(label, val string) {
		sb.WriteString(styleKey.Render(padRight(label, 14)))
		sb.WriteString(val + "\n")
	}

	sb.WriteString(styleTitle.Render(p.Name) + "  " + styleDimmed.Render(p.Version) + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n\n")

	if p.Description != "" {
		sb.WriteString(styleDimmed.Render(wrapText(p.Description, m.detailWidth()-8)) + "\n\n")
	}

	if p.Size > 0 {
		write("Size", styleVal.Render(p.FormatSize()))
	}
	if !p.InstallDate.IsZero() {
		write("Installed", styleVal.Render(p.InstallDate.Format("Jan 02, 2006")))
	}
	write("Reason", reasonLabel(p))
	if p.Architecture != "" {
		write("Arch", styleDimmed.Render(p.Architecture))
	}

	if len(p.Dependencies) > 0 {
		deps := strings.Join(p.Dependencies[:minI(len(p.Dependencies), 8)], "  ")
		if len(p.Dependencies) > 8 {
			deps += "  +" + itoa(len(p.Dependencies)-8) + " more"
		}
		sb.WriteString("\n" + styleKey.Render("Dependencies") + "\n")
		sb.WriteString(styleDimmed.Render("  "+wrapText(deps, m.detailWidth()-10)) + "\n")
	}

	if len(p.RequiredBy) > 0 {
		sb.WriteString("\n" + styleKey.Render("Required By") + "\n")
		sb.WriteString(styleExplicit.Render("  "+strings.Join(p.RequiredBy, "  ")) + "\n")
	}

	sb.WriteString("\n" + styleKey.Render("Health") + "\n")
	sb.WriteString(renderHealthBar(p.HealthScore) + "\n")

	sb.WriteString("\n" + styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n")
	sb.WriteString(styleAILabel.Render("AI Analysis") + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n\n")

	switch {
	case m.aiLoading:
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Analyzing...") + "\n")
	case m.aiErr != "":
		sb.WriteString(styleOrphan.Render(m.aiErr) + "\n")
	case m.aiText != "":
		body, verdict := splitVerdict(m.aiText)
		sb.WriteString(styleVal.Render(wrapText(body, m.detailWidth()-8)) + "\n\n")
		if verdict != "" {
			sb.WriteString(styleVerdict.Render(verdict) + "\n")
		}
	default:
		sb.WriteString(styleDimmed.Render("Press a to analyze with AI") + "\n")
	}

	return sb.String()
}

func renderHealthBar(score int) string {
	filled := score / 10
	empty := 10 - filled
	bar := strings.Repeat("●", filled) + strings.Repeat("○", empty)
	label := " " + itoa(score) + "/100"

	var colored string
	switch {
	case score >= 70:
		colored = styleHealthHigh.Render(bar)
	case score >= 40:
		colored = styleHealthMid.Render(bar)
	default:
		colored = styleHealthLow.Render(bar)
	}
	return colored + styleDimmed.Render(label)
}

func reasonLabel(p *pm.Package) string {
	if p.IsOrphan {
		return styleOrphan.Render("ORPHAN — no longer required")
	}
	if p.InstallReason == "Explicitly installed" {
		return styleExplicit.Render("Explicit")
	}
	return styleDimmed.Render("Dependency")
}

func splitVerdict(text string) (string, string) {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "Verdict:") {
			return strings.Join(lines[:i], "\n"), strings.TrimSpace(lines[i])
		}
	}
	return text, ""
}

func wrapText(s string, width int) string {
	if width <= 0 {
		return s
	}
	var out strings.Builder
	words := strings.Fields(s)
	col := 0
	for i, w := range words {
		if col+len(w)+1 > width && col > 0 {
			out.WriteByte('\n')
			col = 0
		} else if i > 0 {
			out.WriteByte(' ')
			col++
		}
		out.WriteString(w)
		col += len(w)
	}
	return out.String()
}

func padRight(s string, n int) string {
	for len(s) < n {
		s += " "
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	pos := len(b)
	for n > 0 {
		pos--
		b[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

func minI(a, b int) int {
	if a < b {
		return a
	}
	return b
}
