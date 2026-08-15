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
		if msg.BatchID != m.batchID {
			return m, nil
		}
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
		if msg.BatchID != m.batchID {
			return m, nil
		}
		if msg.CurrentIdx >= len(msg.MissingPkgs) {
			return m, func() tea.Msg { return BatchProgressMsg{BatchID: msg.BatchID, Done: true} }
		}

		pkg := msg.MissingPkgs[msg.CurrentIdx]

		progressCmd := func() tea.Msg {
			return BatchProgressMsg{
				BatchID: msg.BatchID,
				Total:   len(msg.MissingPkgs),
				Current: msg.CurrentIdx,
				PkgName: pkg.Name,
			}
		}

		analyzeCmd := func() tea.Msg {
			key := pkg.Name + "@" + pkg.Version
			// Re-check cache right before making the call — a parallel run may have
			// already populated this entry between when the batch was built and now.
			if _, ok := m.cache.Get(key); !ok {
				text, err := m.aiSvc.Analyze(context.Background(), &pkg, msg.ExplicitNames)
				if err == nil {
					m.cache.Set(key, text)
					time.Sleep(2500 * time.Millisecond) // 2.5s pacing (24 RPM) to respect API rate limits
				} else {
					// On rate limit or error, pause background batch for 10s before attempting next item
					time.Sleep(10 * time.Second)
				}
			}

			return processNextBatchPkgMsg{
				BatchID:       msg.BatchID,
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

		// Advance batchID so any previous batch loop stops processing,
		// and start a fresh batch analysis for current manager's packages.
		m.batchID++
		m.batchActive = true
		return m, triggerBatchAnalysis(m.batchID, m.cache, m.allPkgs)

	case pkgDetailMsg:
		if msg.err == nil && msg.pkg != nil {
			m.selectedPkg = msg.pkg
			m.removeErr = ""
			// Auto-analyze now that we have the full package detail
			cmd := m.autoAnalyze()
			m.detailVP.SetContent(m.buildDetailContent())
			m.detailVP.GotoTop()
			return m, cmd
		}
		return m, nil

	case aiAnalysisMsg:
		if m.selectedPkg != nil && (m.selectedPkg.Name+"@"+m.selectedPkg.Version) == msg.pkgKey {
			m.aiLoading = false
			if msg.err != nil {
				m.aiErr = "AI unavailable: " + msg.err.Error()
			} else {
				m.aiText = msg.text
				m.aiErr = ""
			}
			m.detailVP.SetContent(m.buildDetailContent())
		}
		return m, nil

	case pkgRemovedMsg:
		m.removingLoading = false
		if msg.err != nil {
			m.removeErr = msg.err.Error()
			if !m.removingOrphans {
				if len(m.selectedPkgs) > 1 {
					m.detailVP.SetContent(m.buildBatchDetailContent())
				} else if m.selectedPkg != nil {
					m.detailVP.SetContent(m.buildDetailContent())
				}
			}
			return m, nil
		}
		m.removingOrphans = false
		m.checkingOrphans = false
		m.orphanList = nil
		m.selectedPkgs = make(map[string]bool)
		m.loading = true
		m.selectedPkg = nil
		m.focusedPanel = panelList
		return m, loadPackages(m.managers[m.activeMgr])

	case pkgInstalledMsg:
		m.installingLoading = false
		if msg.err != nil {
			m.installErr = msg.err.Error()
			return m, nil
		}
		// Success: close modal and reload packages
		m = m.resetInstallModal()
		m.loading = true
		m.selectedPkg = nil
		m.focusedPanel = panelList
		return m, loadPackages(m.managers[m.activeMgr])

	case pkgSearchResultMsg:
		m.installSearching = false
		if msg.err != nil {
			m.installSearchErr = "Search failed: " + msg.err.Error()
			m.installPkgInput.Focus()
			return m, nil
		}
		if len(msg.results) == 0 {
			m.installSearchErr = "No results for \"" + m.installPkgInput.Value() + "\""
			m.installPkgInput.Focus()
			return m, nil
		}
		m.installResults = msg.results
		m.installResultsCursor = 0
		m.installResultsOffset = 0
		return m, nil

	case orphansCheckedMsg:
		m.checkingOrphans = false
		if msg.err != nil {
			m.removeErr = msg.err.Error()
			return m, nil
		}
		m.orphanList = msg.orphans
		if len(msg.orphans) > 0 {
			m.askingPassword = true
			m.passwordInput.Focus()
			m.passwordInput.SetValue("")
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if key == "q" && !m.searching && !m.askingPassword && !m.installingModal {
			return m, tea.Quit
		}

		// --- Install modal input handling ---
		if m.installingModal {
			return m.handleInstallModalKey(key, msg)
		}

		if m.askingPassword {
			switch key {
			case "esc":
				m.askingPassword = false
				m.removingOrphans = false
				m.passwordInput.Blur()
				m.passwordInput.SetValue("")
				if len(m.selectedPkgs) > 1 {
					m.detailVP.SetContent(m.buildBatchDetailContent())
				} else if m.selectedPkg != nil {
					m.detailVP.SetContent(m.buildDetailContent())
				} else {
					m.detailVP.SetContent(m.renderDetailEmpty())
				}
				return m, nil
			case "enter":
				m.askingPassword = false
				m.removingLoading = true
				m.removeErr = ""
				pw := m.passwordInput.Value()
				m.passwordInput.Blur()
				m.passwordInput.SetValue("")

				var cmdArgs []string
				if m.removingOrphans {
					cmdArgs = m.managers[m.activeMgr].UninstallOrphansCmd()
				} else {
					targets := m.getSelectedNames()
					if len(targets) > 1 {
						m.detailVP.SetContent(m.buildBatchDetailContent())
					} else {
						m.detailVP.SetContent(m.buildDetailContent())
					}
					cmdArgs = m.managers[m.activeMgr].UninstallCmd(targets)
				}

				return m, removePackageCmdAsync(cmdArgs, pw, m.managers[m.activeMgr].RequiresSudo())
			default:
				var cmd tea.Cmd
				m.passwordInput, cmd = m.passwordInput.Update(msg)
				if !m.removingOrphans {
					if len(m.selectedPkgs) > 1 {
						m.detailVP.SetContent(m.buildBatchDetailContent())
					} else {
						m.detailVP.SetContent(m.buildDetailContent())
					}
				}
				return m, cmd
			}
		}

		if m.removingOrphans {
			switch key {
			case "esc":
				m.removingOrphans = false
				m.checkingOrphans = false
				m.askingPassword = false
				m.removingLoading = false
				m.orphanList = nil
				m.removeErr = ""
				m.passwordInput.Blur()
				m.passwordInput.SetValue("")
				return m, nil
			case "o":
				if !m.removingLoading && !m.checkingOrphans {
					if len(m.orphanList) > 0 {
						m.askingPassword = true
						m.removeErr = ""
						m.passwordInput.Focus()
						m.passwordInput.SetValue("")
						return m, nil
					}
				}
			}
			return m, nil
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

func (m Model) startRemoval() (Model, tea.Cmd) {
	targets := m.getSelectedNames()
	if len(targets) == 0 || m.removingLoading {
		return m, nil
	}

	m.removeErr = ""

	// Ensure selectedPkg is updated if targeting a single package from list cursor
	if len(targets) == 1 {
		pkgName := targets[0]
		if m.selectedPkg == nil || m.selectedPkg.Name != pkgName {
			for i := range m.filteredPkgs {
				if m.filteredPkgs[i].Name == pkgName {
					m.selectedPkg = &m.filteredPkgs[i]
					break
				}
			}
		}
	}

	if m.managers[m.activeMgr].RequiresSudo() {
		// Pacman: ask for sudo password first
		m.askingPassword = true
		m.passwordInput.Focus()
		m.passwordInput.SetValue("")
		if len(targets) > 1 {
			m.detailVP.SetContent(m.buildBatchDetailContent())
		} else {
			m.detailVP.SetContent(m.buildDetailContent())
		}
		m.lastKey = ""
		return m, nil
	}

	// Flatpak / npm: run directly, no password needed
	m.removingLoading = true
	cmdArgs := m.managers[m.activeMgr].UninstallCmd(targets)
	if len(targets) > 1 {
		m.detailVP.SetContent(m.buildBatchDetailContent())
	} else {
		m.detailVP.SetContent(m.buildDetailContent())
	}
	m.lastKey = ""
	return m, removePackageCmdAsync(cmdArgs, "", false)
}

func (m Model) handleDetailKey(key string) (Model, tea.Cmd) {
	switch key {
	case "a":
		// Manual retry — only useful if auto-analysis errored
		if m.selectedPkg != nil && !m.aiLoading && m.aiErr != "" {
			cmd := m.autoAnalyze()
			m.detailVP.SetContent(m.buildDetailContent())
			m.lastKey = ""
			return m, cmd
		}
	case "x":
		return m.startRemoval()
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
	case "o":
		return m, m.startOrphanRemoval()
	case "i":
		return m.startInstall()
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
	case "o":
		return m, m.startOrphanRemoval()
	case "i":
		return m.startInstall()
	case "l", "right", "enter":
		m.focusedPanel = panelList
	}
	m.lastKey = ""
	return m, nil
}

func (m Model) handleListKey(key string) (Model, tea.Cmd) {
	switch key {
	case "x":
		return m.startRemoval()
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
	case "/", "?":
		m.searching = true
		m.searchInput.Focus()
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
	case "o":
		return m, m.startOrphanRemoval()
	case "i":
		return m.startInstall()
	default:
		m.lastKey = ""
	}
	return m, nil
}

func (m *Model) triggerSelect() tea.Cmd {
	m.removingOrphans = false
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

	if pkg.Size == 0 {
		// Need full detail first — analysis will be triggered in pkgDetailMsg handler
		m.detailVP.SetContent(m.buildDetailContent())
		m.detailVP.GotoTop()
		return loadPackageDetail(m.managers[m.activeMgr], pkg.Name)
	}

	// Auto-analyze: serve from cache instantly, or fire the request
	cmd := m.autoAnalyze()
	m.detailVP.SetContent(m.buildDetailContent())
	m.detailVP.GotoTop()
	return cmd
}

// autoAnalyze checks the cache for the current selectedPkg and either
// populates aiText immediately (cache hit) or fires analyzePackage (miss).
// It must be called after selectedPkg is set.
func (m *Model) autoAnalyze() tea.Cmd {
	if m.selectedPkg == nil {
		return nil
	}
	key := m.selectedPkg.Name + "@" + m.selectedPkg.Version
	if text, ok := m.cache.Get(key); ok {
		m.aiText = text
		m.aiLoading = false
		return nil
	}
	// Not cached yet — fire background analysis
	m.aiLoading = true
	m.aiText = ""
	m.aiErr = ""
	var explicitNames []string
	for _, p := range m.allPkgs {
		if p.InstallReason == "Explicitly installed" {
			explicitNames = append(explicitNames, p.Name)
		}
	}
	return analyzePackage(m.aiSvc, m.cache, m.selectedPkg, explicitNames)
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
		sb.WriteString(styleDimmed.Render("Press a to retry") + "\n")
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
		sb.WriteString(styleDimmed.Render("Press x to remove") + "\n")
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

func checkOrphansCmd(mgr pm.Manager) tea.Cmd {
	return func() tea.Msg {
		orphans, err := mgr.GetOrphans()
		return orphansCheckedMsg{orphans: orphans, err: err}
	}
}

func (m *Model) startOrphanRemoval() tea.Cmd {
	if !m.removingLoading && !m.aiLoading && !m.checkingOrphans {
		m.removingOrphans = true
		m.checkingOrphans = true
		m.askingPassword = false
		m.orphanList = nil
		m.removeErr = ""
		m.passwordInput.Blur()
		m.passwordInput.SetValue("")
		m.lastKey = ""
		return checkOrphansCmd(m.managers[m.activeMgr])
	}
	return nil
}

func (m Model) resetInstallModal() Model {
	m.installingModal = false
	m.installAskPassword = false
	m.installingLoading = false
	m.installErr = ""
	m.installSearchErr = ""
	m.installPkgName = ""
	m.installResults = nil
	m.installResultsCursor = 0
	m.installResultsOffset = 0
	m.installSearching = false
	m.installShowDesc = false
	m.installPkgInput.Blur()
	m.installPkgInput.SetValue("")
	m.installPasswordInput.Blur()
	m.installPasswordInput.SetValue("")
	return m
}

func (m Model) startInstall() (Model, tea.Cmd) {
	if !m.installingModal && !m.installingLoading {
		m = m.resetInstallModal()
		m.installingModal = true
		m.installPkgInput.Focus()
		m.lastKey = ""
	}
	return m, nil
}

// handleInstallModalKey is the keyboard handler for the install modal.
// The modal has five phases driven by state flags:
//
//	search input → (Enter) → searching → results list → (Enter) → password → (Enter) → installing
//
// Esc navigates backwards through the phases.
func (m Model) handleInstallModalKey(key string, msg tea.KeyMsg) (Model, tea.Cmd) {
	const listVisibleH = 8 // rows of results shown at once

	// ── Phase: installing (spinner) ──────────────────────────────────────
	if m.installingLoading {
		// Nothing to do while installing; Esc would be confusing here.
		return m, nil
	}

	// ── Phase: install error ─────────────────────────────────────────────
	if m.installErr != "" {
		switch key {
		case "esc", "enter":
			// Reset to search so the user can try something else
			m.installErr = ""
			m.installResults = nil
			m.installResultsCursor = 0
			m.installResultsOffset = 0
			m.installPkgInput.Focus()
		}
		return m, nil
	}

	// ── Phase: password input ────────────────────────────────────────────
	if m.installAskPassword {
		switch key {
		case "esc":
			// Go back to results list (or search if no results)
			m.installAskPassword = false
			m.installPasswordInput.Blur()
			m.installPasswordInput.SetValue("")
		case "enter":
			pw := m.installPasswordInput.Value()
			m.installAskPassword = false
			m.installingLoading = true
			m.installErr = ""
			m.installPasswordInput.Blur()
			m.installPasswordInput.SetValue("")
			cmdArgs := m.managers[m.activeMgr].InstallCmd(m.installPkgName)
			needsSudo := m.managers[m.activeMgr].RequiresSudo()
			return m, installPackageCmdAsync(cmdArgs, pw, needsSudo)
		default:
			var cmd tea.Cmd
			m.installPasswordInput, cmd = m.installPasswordInput.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// ── Phase: package description popup ────────────────────────────────
	if m.installShowDesc && len(m.installResults) > 0 {
		switch key {
		case "esc", "tab":
			m.installShowDesc = false
		case "enter":
			m.installShowDesc = false
			pkg := m.installResults[m.installResultsCursor]
			m.installPkgName = pkg.Name
			if m.managers[m.activeMgr].RequiresSudo() {
				// Ask for sudo password before installing
				m.installAskPassword = true
				m.installPasswordInput.Focus()
				m.installPasswordInput.SetValue("")
			} else {
				// Flatpak / npm — run directly, no password needed
				m.installingLoading = true
				m.installErr = ""
				cmdArgs := m.managers[m.activeMgr].InstallCmd(m.installPkgName)
				return m, installPackageCmdAsync(cmdArgs, "", false)
			}
		}
		return m, nil
	}

	// ── Phase: results list ──────────────────────────────────────────────
	if len(m.installResults) > 0 {
		switch key {
		case "tab":
			m.installShowDesc = true
		case "esc", "/":
			// Back to search input
			m.installResults = nil
			m.installResultsCursor = 0
			m.installResultsOffset = 0
			m.installSearchErr = ""
			m.installShowDesc = false
			m.installPkgInput.Focus()
		case "j", "down":
			if m.installResultsCursor < len(m.installResults)-1 {
				m.installResultsCursor++
				if m.installResultsCursor >= m.installResultsOffset+listVisibleH {
					m.installResultsOffset = m.installResultsCursor - listVisibleH + 1
				}
			}
		case "k", "up":
			if m.installResultsCursor > 0 {
				m.installResultsCursor--
				if m.installResultsCursor < m.installResultsOffset {
					m.installResultsOffset = m.installResultsCursor
				}
			}
		case "enter":
			pkg := m.installResults[m.installResultsCursor]
			m.installPkgName = pkg.Name
			if m.managers[m.activeMgr].RequiresSudo() {
				// Ask for sudo password before installing
				m.installAskPassword = true
				m.installPasswordInput.Focus()
				m.installPasswordInput.SetValue("")
			} else {
				// Flatpak / npm — run directly, no password needed
				m.installingLoading = true
				m.installErr = ""
				cmdArgs := m.managers[m.activeMgr].InstallCmd(m.installPkgName)
				return m, installPackageCmdAsync(cmdArgs, "", false)
			}
		}
		return m, nil
	}

	// ── Phase: searching (spinner) ───────────────────────────────────────
	if m.installSearching {
		if key == "esc" {
			m.installSearching = false
			m.installPkgInput.Focus()
		}
		return m, nil
	}

	// ── Phase: search error ──────────────────────────────────────────────
	if m.installSearchErr != "" {
		switch key {
		case "esc":
			m.installSearchErr = ""
			m.installPkgInput.Focus()
		case "enter":
			// Retry the same query
			query := strings.TrimSpace(m.installPkgInput.Value())
			if query == "" {
				m.installSearchErr = ""
				m.installPkgInput.Focus()
				return m, nil
			}
			m.installSearchErr = ""
			m.installSearching = true
			m.installPkgInput.Blur()
			return m, searchPackagesCmd(m.managers[m.activeMgr], query)
		}
		return m, nil
	}

	// ── Phase: search input (default) ────────────────────────────────────
	switch key {
	case "esc":
		m = m.resetInstallModal()
	case "enter":
		query := strings.TrimSpace(m.installPkgInput.Value())
		if len(query) < 2 {
			return m, nil // require at least 2 characters
		}
		m.installSearching = true
		m.installPkgInput.Blur()
		return m, searchPackagesCmd(m.managers[m.activeMgr], query)
	default:
		var cmd tea.Cmd
		m.installPkgInput, cmd = m.installPkgInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func searchPackagesCmd(mgr pm.Manager, query string) tea.Cmd {
	return func() tea.Msg {
		results, err := mgr.Search(query)
		if err == nil && len(results) > 0 {
			results = pm.RankSearchResults(query, results)
		}
		return pkgSearchResultMsg{results: results, err: err}
	}
}


func installPackageCmdAsync(cmdArgs []string, password string, needsSudo bool) tea.Cmd {
	return func() tea.Msg {
		var c *exec.Cmd
		if needsSudo {
			// Pipe the password to sudo -S via stdin
			args := append([]string{"-S"}, cmdArgs...)
			c = exec.Command("sudo", args...)
			c.Stdin = strings.NewReader(password + "\n")
		} else {
			// Flatpak and npm handle their own permissions — never wrap in sudo
			c = exec.Command(cmdArgs[0], cmdArgs[1:]...)
		}
		out, err := c.CombinedOutput()
		if err != nil {
			return pkgInstalledMsg{err: fmt.Errorf("%v: %s", err, out)}
		}
		return pkgInstalledMsg{err: nil}
	}
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

func removePackageCmdAsync(cmdArgs []string, password string, needsSudo bool) tea.Cmd {
	return func() tea.Msg {
		var c *exec.Cmd
		if needsSudo {
			args := append([]string{"-S"}, cmdArgs...)
			c = exec.Command("sudo", args...)
			c.Stdin = strings.NewReader(password + "\n")
		} else {
			// Flatpak and npm handle their own permissions
			c = exec.Command(cmdArgs[0], cmdArgs[1:]...)
		}
		out, err := c.CombinedOutput()
		if err != nil {
			return pkgRemovedMsg{err: fmt.Errorf("%v: %s", err, out)}
		}
		return pkgRemovedMsg{err: nil}
	}
}

func triggerBatchAnalysis(batchID uint64, c *cache.Cache, pkgs []pm.Package) tea.Cmd {
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
			return BatchProgressMsg{BatchID: batchID, Done: true}
		}

		return processNextBatchPkgMsg{
			BatchID:       batchID,
			MissingPkgs:   missing,
			CurrentIdx:    0,
			ExplicitNames: explicitNames,
		}
	}
}
