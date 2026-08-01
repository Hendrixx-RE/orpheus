package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"context"
	"time"

	"orpheus/internal/pm"
	"orpheus/internal/cache"

	"github.com/charmbracelet/bubbles/progress"
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
		// re-render detail content to animate the spinner while AI or removing is loading
		if (m.aiLoading || m.removingLoading) && m.selectedPkg != nil {
			m.detailVP.SetContent(m.buildDetailContent())
		}
		return m, cmd

	case BatchProgressMsg:
		m.batchActive = !msg.Done
		m.batchTotal = msg.Total
		m.batchCurrent = msg.Current
		m.batchPkg = msg.PkgName
		var cmd tea.Cmd
		if m.batchTotal > 0 {
			percent := float64(m.batchCurrent) / float64(m.batchTotal)
			cmd = m.progress.SetPercent(percent)
		}
		return m, cmd

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case processNextBatchPkgMsg:
		if msg.CurrentIdx >= len(msg.MissingPkgs) {
			return m, func() tea.Msg { return BatchProgressMsg{Done: true} }
		}

		pkg := msg.MissingPkgs[msg.CurrentIdx]

		progressCmd := func() tea.Msg {
			return BatchProgressMsg{
				Total:   len(msg.MissingPkgs),
				Current: msg.CurrentIdx,
				PkgName: pkg.Name,
			}
		}

		analyzeCmd := func() tea.Msg {
			key := pkg.Name + "@" + pkg.Version
			text, err := m.aiSvc.Analyze(context.Background(), &pkg, msg.ExplicitNames)
			if err == nil {
				m.cache.Set(key, text)
			}
			
			time.Sleep(200 * time.Millisecond) // slight delay to avoid hammering API too hard

			return processNextBatchPkgMsg{
				MissingPkgs:   msg.MissingPkgs,
				CurrentIdx:    msg.CurrentIdx + 1,
				ExplicitNames: msg.ExplicitNames,
			}
		}

		return m, tea.Sequence(progressCmd, analyzeCmd)

	case aiSearchResultMsg:
		if msg.Err != nil {
			// fallback/clear if error
			m.applyFilter()
		} else if msg.PkgNames == nil {
			// clear AI search
			m.applyFilter()
		} else {
			// filter packages by the ones found in cache
			nameMap := make(map[string]bool)
			for _, n := range msg.PkgNames {
				nameMap[n] = true
			}
			var out []pm.Package
			for _, p := range m.allPkgs {
				if p.InstallReason == "Explicitly installed" && nameMap[p.Name] {
					out = append(out, p)
				}
			}
			m.filteredPkgs = out
			m.listCursor = 0
			m.ensureVisible()
		}
		return m, nil

	case pkgsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.allPkgs = msg.pkgs
		m.filteredPkgs = msg.pkgs
		m.applyFilter()

		// Trigger background batch analysis for any un-analyzed explicit packages
		return m, triggerBatchAnalysis(m.cache, m.allPkgs)

	case pkgDetailMsg:
		if msg.err == nil && msg.pkg != nil {
			m.selectedPkg = msg.pkg
			m.removeErr = "" // clear any previous error
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

	case pkgRemovedMsg:
		m.removingLoading = false
		if msg.err != nil {
			m.removeErr = msg.err.Error()
			if len(m.selectedPkgs) > 1 {
				m.detailVP.SetContent(m.buildBatchDetailContent())
			} else if m.selectedPkg != nil {
				m.detailVP.SetContent(m.buildDetailContent())
			}
			return m, nil
		}
		m.loading = true
		m.selectedPkg = nil
		m.focusedPanel = panelList
		return m, loadPackages(m.managers[m.activeMgr])

	case tea.KeyMsg:
		key := msg.String()

		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if key == "q" && !m.searching && !m.aiSearching && !m.askingPassword {
			return m, tea.Quit
		}

		if m.askingPassword {
			switch key {
			case "esc":
				m.askingPassword = false
				m.passwordInput.Blur()
				m.passwordInput.SetValue("")
				if len(m.selectedPkgs) > 1 {
					m.detailVP.SetContent(m.buildBatchDetailContent())
				} else {
					m.detailVP.SetContent(m.buildDetailContent())
				}
				return m, nil
			case "enter":
				m.askingPassword = false
				m.removingLoading = true
				m.removeErr = ""
				pw := m.passwordInput.Value()
				m.passwordInput.Blur()
				m.passwordInput.SetValue("")

				targets := m.getSelectedNames()
				if len(targets) > 1 {
					m.detailVP.SetContent(m.buildBatchDetailContent())
				} else {
					m.detailVP.SetContent(m.buildDetailContent())
				}

				cmdArgs := m.managers[m.activeMgr].UninstallCmd(targets)
				return m, removePackageCmdAsync(cmdArgs, pw)
			default:
				var cmd tea.Cmd
				m.passwordInput, cmd = m.passwordInput.Update(msg)
				if len(m.selectedPkgs) > 1 {
					m.detailVP.SetContent(m.buildBatchDetailContent())
				} else {
					m.detailVP.SetContent(m.buildDetailContent())
				}
				return m, cmd
			}
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

		if m.aiSearching {
			switch key {
			case "esc":
				m.aiSearching = false
				m.aiSearchInput.Blur()
				m.aiSearchInput.SetValue("")
			case "enter":
				query := m.aiSearchInput.Value()
				m.aiSearching = false
				m.aiSearchInput.Blur()
				return m, runRipgrepCmd(query, m.cache)
			default:
				var cmd tea.Cmd
				m.aiSearchInput, cmd = m.aiSearchInput.Update(msg)
				return m, cmd
			}
			return m, nil
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

			var explicitNames []string
			for _, p := range m.allPkgs {
				if p.InstallReason == "Explicitly installed" {
					explicitNames = append(explicitNames, p.Name)
				}
			}
			return m, analyzePackage(m.aiSvc, m.cache, m.selectedPkg, explicitNames)
		}
	case "x":
		if (m.selectedPkg != nil || len(m.selectedPkgs) > 1) && !m.removingLoading && !m.aiLoading {
			m.askingPassword = true
			m.removeErr = ""
			m.passwordInput.Focus()
			m.passwordInput.SetValue("")
			if len(m.selectedPkgs) > 1 {
				m.detailVP.SetContent(m.buildBatchDetailContent())
			} else {
				m.detailVP.SetContent(m.buildDetailContent())
			}
			m.lastKey = ""
			return m, nil
		}
	case "j", "down":
		m.detailVP.ScrollDown(1)
	case "k", "up":
		m.detailVP.ScrollUp(1)
	case "ctrl+d":
		m.detailVP.HalfPageDown()
	case "ctrl+u":
		m.detailVP.HalfPageUp()
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
		m.activeMgr = (m.activeMgr + 1) % len(m.managers)
		m.loading = true
		m.selectedPkgs = make(map[string]bool)
		m.visualMode = false
		return m, loadPackages(m.managers[m.activeMgr])
	case "k", "up":
		m.activeMgr--
		if m.activeMgr < 0 {
			m.activeMgr = len(m.managers) - 1
		}
		m.loading = true
		m.selectedPkgs = make(map[string]bool)
		m.visualMode = false
		return m, loadPackages(m.managers[m.activeMgr])
	case "l", "right", "enter":
		m.focusedPanel = panelList
	}
	m.lastKey = ""
	return m, nil
}

func (m Model) handleListKey(key string) (Model, tea.Cmd) {
	switch key {
	case " ":
		m.commitVisualSelection()
		if len(m.filteredPkgs) > 0 {
			pkg := m.filteredPkgs[m.listCursor].Name
			if m.selectedPkgs[pkg] {
				delete(m.selectedPkgs, pkg)
			} else {
				m.selectedPkgs[pkg] = true
			}
		}
		m.lastKey = ""
	case "v":
		if m.visualMode {
			m.commitVisualSelection()
		} else {
			m.visualMode = true
			m.visualStart = m.listCursor
		}
		m.lastKey = ""
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
	case "?":
		m.aiSearching = true
		m.aiSearchInput.Focus()
		m.lastKey = ""
	case "esc":
		if m.visualMode {
			m.visualMode = false
			m.lastKey = ""
			return m, nil
		}
		if len(m.selectedPkgs) > 0 {
			m.selectedPkgs = make(map[string]bool)
			m.lastKey = ""
			return m, nil
		}
		if m.searchInput.Value() != "" {
			m.searchInput.SetValue("")
			m.applyFilter()
		}
		m.lastKey = ""
	case "s":
		m.sortMode = (m.sortMode + 1) % 3
		m.applyFilter()
		m.lastKey = ""
	case "enter", "l", "right":
		m.commitVisualSelection()
		cmd := m.triggerSelect()
		m.lastKey = ""
		return m, cmd
	case "r":
		m.loading = true
		m.lastKey = ""
		return m, loadPackages(m.managers[m.activeMgr])
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
	m.focusedPanel = panelDetail

	targets := m.getSelectedNames()
	if len(targets) > 1 {
		m.selectedPkg = nil
		m.removeErr = ""
		m.detailVP.SetContent(m.buildBatchDetailContent())
		m.detailVP.GotoTop()
		return nil
	}

	pkgName := targets[0]
	var pkg *pm.Package
	for i := range m.filteredPkgs {
		if m.filteredPkgs[i].Name == pkgName {
			pkg = &m.filteredPkgs[i]
			break
		}
	}
	if pkg == nil {
		return nil
	}

	// If already selected same package, just focus detail
	if m.selectedPkg != nil && m.selectedPkg.Name == pkg.Name && m.selectedPkg.Size > 0 {
		m.detailVP.GotoTop()
		return nil
	}

	// Show partial info immediately
	m.selectedPkg = pkg
	m.aiText = ""
	m.aiErr = ""
	m.aiLoading = false
	m.detailVP.SetContent(m.buildDetailContent())
	m.detailVP.GotoTop()

	if pkg.Size == 0 {
		return loadPackageDetail(m.managers[m.activeMgr], pkg.Name)
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
	write("Reason", styleExplicit.Render("Explicit"))
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

	sb.WriteString("\n" + styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n")
	sb.WriteString(styleAILabel.Render("Action Status") + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n\n")

	switch {
	case m.askingPassword:
		sb.WriteString(styleVal.Render("Enter sudo password to remove "+p.Name+":") + "\n")
		sb.WriteString(m.passwordInput.View() + "\n")
		sb.WriteString(styleDimmed.Render("(Press Esc to cancel)") + "\n")
	case m.removingLoading:
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Removing package...") + "\n")
	case m.removeErr != "":
		sb.WriteString(styleOrphan.Render("Removal failed: "+m.removeErr) + "\n")
		sb.WriteString(styleDimmed.Render("Press x to retry") + "\n")
	case m.aiLoading:
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Analyzing...") + "\n")
	case m.aiErr != "":
		sb.WriteString(styleOrphan.Render(m.aiErr) + "\n")
	case m.aiText != "":
		body, verdict, command := splitVerdict(m.aiText)
		sb.WriteString(styleVal.Render(wrapText(strings.TrimSpace(body), m.detailWidth()-8)) + "\n\n")
		if verdict != "" {
			sb.WriteString(styleVerdict.Render(verdict) + "\n")
		}
		if command != "" {
			sb.WriteString(styleAILabel.Render(command) + "\n")
		}
	default:
		sb.WriteString(styleDimmed.Render("Press a to analyze, x to remove") + "\n")
	}

	return sb.String()
}

func (m Model) buildBatchDetailContent() string {
	var sb strings.Builder

	names := m.getSelectedNames()
	count := len(names)

	sb.WriteString(styleTitle.Render("Batch Operation") + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n\n")

	sb.WriteString(styleVal.Render(fmt.Sprintf("%d packages selected", count)) + "\n\n")

	// list first few names
	for i, name := range names {
		if i >= 10 {
			sb.WriteString(styleDimmed.Render(fmt.Sprintf("... and %d more", count-10)) + "\n")
			break
		}
		sb.WriteString("  " + name + "\n")
	}

	sb.WriteString("\n" + styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n")
	sb.WriteString(styleAILabel.Render("Action Status") + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n\n")

	switch {
	case m.askingPassword:
		sb.WriteString(styleVal.Render(fmt.Sprintf("Enter sudo password to remove %d packages:", count)) + "\n")
		sb.WriteString(m.passwordInput.View() + "\n")
		sb.WriteString(styleDimmed.Render("(Press Esc to cancel)") + "\n")
	case m.removingLoading:
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Removing packages...") + "\n")
	case m.removeErr != "":
		sb.WriteString(styleOrphan.Render("Removal failed: "+m.removeErr) + "\n")
		sb.WriteString(styleDimmed.Render("Press x to retry") + "\n")
	default:
		sb.WriteString(styleDimmed.Render("Press x to remove all selected packages") + "\n")
	}

	return sb.String()
}

func splitVerdict(text string) (string, string, string) {
	lines := strings.Split(text, "\n")
	var bodyLines []string
	verdict := ""
	command := ""

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "Verdict:") {
			verdict = line
		} else if strings.HasPrefix(line, "(Command:") {
			command = line
		} else {
			bodyLines = append(bodyLines, rawLine)
		}
	}
	return strings.Join(bodyLines, "\n"), verdict, command
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

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func removePackageCmdAsync(cmdArgs []string, password string) tea.Cmd {
	return func() tea.Msg {
		args := append([]string{"-S"}, cmdArgs...)
		c := exec.Command("sudo", args...)
		c.Stdin = strings.NewReader(password + "\n")
		out, err := c.CombinedOutput()
		if err != nil {
			return pkgRemovedMsg{err: fmt.Errorf("%v: %s", err, out)}
		}
		return pkgRemovedMsg{err: nil}
	}
}

func triggerBatchAnalysis(c *cache.Cache, pkgs []pm.Package) tea.Cmd {
	return func() tea.Msg {
		var missing []pm.Package
		var explicitNames []string

		for _, p := range pkgs {
			if p.InstallReason == "Explicitly installed" {
				explicitNames = append(explicitNames, p.Name)
				key := p.Name + "@" + p.Version
				if _, ok := c.Get(key); !ok {
					missing = append(missing, p)
				}
			}
		}

		if len(missing) == 0 {
			return BatchProgressMsg{Done: true}
		}

		return processNextBatchPkgMsg{
			MissingPkgs:   missing,
			CurrentIdx:    0,
			ExplicitNames: explicitNames,
		}
	}
}

func runRipgrepCmd(query string, c *cache.Cache) tea.Cmd {
	return func() tea.Msg {
		if query == "" {
			return aiSearchResultMsg{PkgNames: nil} // clears search
		}
		
		cmd := exec.Command("rg", "-i", query, c.Path())
		out, err := cmd.Output()
		if err != nil {
			// rg returns exit status 1 if no matches found
			return aiSearchResultMsg{PkgNames: []string{}}
		}
		
		var matchingNames []string
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// line format: "package@version": "analysis...
			parts := strings.SplitN(line, `"`, 3)
			if len(parts) >= 2 {
				key := parts[1]
				nameVer := strings.SplitN(key, "@", 2)
				if len(nameVer) > 0 {
					matchingNames = append(matchingNames, nameVer[0])
				}
			}
		}
		
		return aiSearchResultMsg{PkgNames: matchingNames}
	}
}
