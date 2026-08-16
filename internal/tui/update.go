package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"context"

	"orpheus/internal/ai"
	"orpheus/internal/cache"
	"orpheus/internal/pm"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.detailVP = viewport.New(m.detailWidth()-6, m.height-6)
		m.installOutputVP = viewport.New(60, 6)
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

	case syncProgressMsg:
		m.syncTotal = msg.Total
		m.syncDone = msg.Done
		m.syncActive = !msg.DoneAll
		if !msg.DoneAll && m.syncChan != nil {
			return m, waitForSyncProgress(m.syncChan)
		}
		return m, nil

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

		// Cancel any running background sync and start a new worker for current manager
		if m.syncCancel != nil {
			m.syncCancel()
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.syncCancel = cancel
		m.syncChan = make(chan syncProgressMsg, 16)
		m.syncActive = true

		go runSyncWorker(ctx, m.aiSvc, m.cache, m.allPkgs, m.syncChan)
		return m, waitForSyncProgress(m.syncChan)

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

	case installAIAnalysisMsg:
		m.installAILoading = false
		if msg.err != nil {
			m.installAIErr = "AI unavailable: " + msg.err.Error()
		} else {
			m.installAIAnalysis = msg.text
			m.installAIErr = ""
		}
		return m, nil

	case pkgInstalledMsg:
		m.installingLoading = false
		if msg.err != nil {
			m.installErr = msg.err.Error()
			m.installOutput = msg.output
			m.installOutputVP.SetContent(msg.output)
			m.installOutputVP.GotoBottom()
			return m, nil
		}
		// Success: close modal and reload packages
		m = m.resetInstallModal()
		m.loading = true
		m.selectedPkg = nil
		m.focusedPanel = panelList
		return m, loadPackages(m.managers[m.activeMgr])

	case pkgUpdateOutputMsg:
		m.updatingLoading = false
		m.updateOutput = msg.output
		m.updateOutputVP.SetContent(msg.output)
		m.updateOutputVP.GotoBottom()
		if msg.err != nil {
			m.updateErr = msg.err.Error()
		} else {
			m.updateDone = true
			m.selectedPkgs = make(map[string]bool)
		}
		return m, nil

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
		if key == "q" && !m.searching && !m.askingPassword && !m.installingModal && !m.updatingModal {
			return m, tea.Quit
		}

		// --- Install modal input handling ---
		if m.installingModal {
			return m.handleInstallModalKey(key, msg)
		}

		// --- Update modal input handling ---
		if m.updatingModal {
			return m.handleUpdateModalKey(key, msg)
		}

		if m.askingPassword {
			switch key {
			case "esc":
				m.askingPassword = false
				m.removingOrphans = false
				m.passwordInput.Blur()
				m.passwordInput.SetValue("")
				if len(m.getSelectedNames()) > 1 {
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
					if len(m.getSelectedNames()) > 1 {
						m.detailVP.SetContent(m.buildBatchDetailContent())
					} else {
						m.detailVP.SetContent(m.buildDetailContent())
					}
					m.detailVP.GotoTop()
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
	m.focusedPanel = panelDetail

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
		m.detailVP.GotoTop()
		m.lastKey = ""
		return m, textinput.Blink
	}

	// Flatpak: run directly, no password needed
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
	case "U":
		return m.startFullUpgrade()
	case "u":
		return m.startUpdate()
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
	case "u", "U":
		return m.startFullUpgrade()
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
	case "U":
		return m.startFullUpgrade()
	case "u":
		return m.startUpdate()
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
	if text, ok := m.cache.GetPackage(m.selectedPkg.Name, m.selectedPkg.Version); ok {
		m.aiText = text
		m.aiLoading = false
		m.aiErr = ""
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

	sb.WriteString(styleTitle.Render("Batch Removal") + "  " + styleDimmed.Render(fmt.Sprintf("(%d packages)", count)) + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n\n")

	if m.askingPassword {
		sb.WriteString(styleOrphan.Render(fmt.Sprintf("Sudo Password Required to Remove %d Packages:", count)) + "\n")
		sb.WriteString(m.passwordInput.View() + "\n")
		sb.WriteString(styleDimmed.Render("Enter to confirm  |  Esc to cancel") + "\n")
		sb.WriteString(styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n\n")
	} else if m.removingLoading {
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render(fmt.Sprintf("Removing %d packages...", count)) + "\n")
		sb.WriteString(styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n\n")
	} else if m.removeErr != "" {
		sb.WriteString(styleOrphan.Render("Removal failed: "+m.removeErr) + "\n")
		sb.WriteString(styleDimmed.Render("Press x to retry") + "\n")
		sb.WriteString(styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n\n")
	}

	// Map package details and calculate total size
	pkgMap := make(map[string]pm.Package, len(m.allPkgs))
	for _, p := range m.allPkgs {
		pkgMap[p.Name] = p
	}

	var totalSize int64
	for _, name := range names {
		if p, ok := pkgMap[name]; ok {
			totalSize += p.Size
		}
	}

	var headerInfo string
	if totalSize > 0 {
		dummy := pm.Package{Size: totalSize}
		headerInfo = fmt.Sprintf("Packages to be uninstalled (%d items, %s):", count, dummy.FormatSize())
	} else {
		headerInfo = fmt.Sprintf("Packages to be uninstalled (%d items):", count)
	}
	sb.WriteString(styleKey.Render(headerInfo) + "\n\n")

	dot := lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render("●")

	for _, name := range names {
		if p, ok := pkgMap[name]; ok {
			sizeStr := ""
			if p.Size > 0 {
				sizeStr = " (" + p.FormatSize() + ")"
			}
			verStr := ""
			if p.Version != "" {
				verStr = " " + styleDimmed.Render(p.Version)
			}
			sb.WriteString("  " + dot + " " + styleVal.Render(name) + verStr + styleDimmed.Render(sizeStr) + "\n")
		} else {
			sb.WriteString("  " + dot + " " + styleVal.Render(name) + "\n")
		}
	}

	sb.WriteString("\n" + styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n")
	sb.WriteString(styleAILabel.Render("Action Status") + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", m.detailWidth()-6)) + "\n\n")

	switch {
	case m.askingPassword:
		sb.WriteString(styleDimmed.Render("Enter sudo password above to proceed.") + "\n")
	case m.removingLoading:
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Uninstalling selected packages...") + "\n")
	case m.removeErr != "":
		sb.WriteString(styleOrphan.Render(m.removeErr) + "\n")
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
	m.installAIAnalysis = ""
	m.installAILoading = false
	m.installAIErr = ""
	m.installOutput = ""
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
		m.installMgrIndex = m.activeMgr
		m.installPkgInput.Focus()
		m.lastKey = ""
	}
	return m, nil
}

// handleInstallModalKey is the keyboard handler for the install modal.
func (m Model) handleInstallModalKey(key string, msg tea.KeyMsg) (Model, tea.Cmd) {
	const listVisibleH = 8 // rows of results shown at once
	activeMgr := m.managers[m.installMgrIndex]

	// ── Phase: installing (spinner) ──────────────────────────────────────
	if m.installingLoading {
		return m, nil
	}

	// ── Phase: install error ─────────────────────────────────────────────
	if m.installErr != "" {
		switch key {
		case "j", "down":
			m.installOutputVP.ScrollDown(1)
		case "k", "up":
			m.installOutputVP.ScrollUp(1)
		case "ctrl+d":
			m.installOutputVP.HalfPageDown()
		case "ctrl+u":
			m.installOutputVP.HalfPageUp()
		case "esc", "enter":
			// Reset to search so the user can try something else
			m.installErr = ""
			m.installOutput = ""
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
			// Go back to results list
			m.installAskPassword = false
			m.installPasswordInput.Blur()
			m.installPasswordInput.SetValue("")
		case "enter":
			pw := m.installPasswordInput.Value()
			m.installAskPassword = false
			m.installingLoading = true
			m.installErr = ""
			m.installOutput = ""
			m.installPasswordInput.Blur()
			m.installPasswordInput.SetValue("")
			cmdArgs := activeMgr.InstallCmd(m.installPkgName)
			needsSudo := activeMgr.RequiresSudo()
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
		pkg := m.installResults[m.installResultsCursor]
		switch key {
		case "a":
			if !m.installAILoading {
				m.installAILoading = true
				m.installAIErr = ""
				return m, installAIAnalyzeCmd(m.aiSvc, m.cache, &pkg)
			}
		case "esc", "tab":
			m.installShowDesc = false
		case "enter":
			m.installShowDesc = false
			m.installPkgName = pkg.Name
			if activeMgr.RequiresSudo() {
				m.installAskPassword = true
				m.installPasswordInput.Focus()
				m.installPasswordInput.SetValue("")
				return m, textinput.Blink
			} else {
				m.installingLoading = true
				m.installErr = ""
				m.installOutput = ""
				cmdArgs := activeMgr.InstallCmd(m.installPkgName)
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
			pkg := m.installResults[m.installResultsCursor]
			key := "summary@" + pkg.Name + "@" + pkg.Version
			if text, ok := m.cache.Get(key); ok {
				m.installAIAnalysis = text
			} else {
				m.installAIAnalysis = ""
			}
			m.installAIErr = ""
		case "esc", "/":
			// Back to search input
			m.installResults = nil
			m.installResultsCursor = 0
			m.installResultsOffset = 0
			m.installSearchErr = ""
			m.installShowDesc = false
			m.installAIAnalysis = ""
			m.installPkgInput.Focus()
		case "j", "down":
			if m.installResultsCursor < len(m.installResults)-1 {
				m.installResultsCursor++
				if m.installResultsCursor >= m.installResultsOffset+listVisibleH {
					m.installResultsOffset = m.installResultsCursor - listVisibleH + 1
				}
				m.installAIAnalysis = ""
				m.installAIErr = ""
			}
		case "k", "up":
			if m.installResultsCursor > 0 {
				m.installResultsCursor--
				if m.installResultsCursor < m.installResultsOffset {
					m.installResultsOffset = m.installResultsCursor
				}
				m.installAIAnalysis = ""
				m.installAIErr = ""
			}
		case "enter":
			pkg := m.installResults[m.installResultsCursor]
			m.installPkgName = pkg.Name
			if activeMgr.RequiresSudo() {
				m.installAskPassword = true
				m.installPasswordInput.Focus()
				m.installPasswordInput.SetValue("")
				return m, textinput.Blink
			} else {
				m.installingLoading = true
				m.installErr = ""
				m.installOutput = ""
				cmdArgs := activeMgr.InstallCmd(m.installPkgName)
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
		case "tab":
			// Cycle manager even on error screen
			m.installMgrIndex = (m.installMgrIndex + 1) % len(m.managers)
		case "esc":
			m.installSearchErr = ""
			m.installPkgInput.Focus()
		case "enter":
			query := strings.TrimSpace(m.installPkgInput.Value())
			if query == "" {
				m.installSearchErr = ""
				m.installPkgInput.Focus()
				return m, nil
			}
			m.installSearchErr = ""
			m.installSearching = true
			m.installPkgInput.Blur()
			return m, searchPackagesCmd(activeMgr, query)
		}
		return m, nil
	}

	// ── Phase: search input (default) ────────────────────────────────────
	switch key {
	case "tab":
		// Switch manager on search screen
		m.installMgrIndex = (m.installMgrIndex + 1) % len(m.managers)
		return m, nil
	case "esc":
		m = m.resetInstallModal()
	case "enter":
		query := strings.TrimSpace(m.installPkgInput.Value())
		if len(query) < 2 {
			return m, nil
		}
		m.installSearching = true
		m.installPkgInput.Blur()
		return m, searchPackagesCmd(activeMgr, query)
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

func installAIAnalyzeCmd(aiSvc *ai.Analyzer, c *cache.Cache, pkg *pm.Package) tea.Cmd {
	return func() tea.Msg {
		key := "summary@" + pkg.Name + "@" + pkg.Version
		if text, ok := c.Get(key); ok {
			return installAIAnalysisMsg{pkgKey: key, text: text}
		}
		text, err := aiSvc.AnalyzeSummary(context.Background(), pkg)
		if err != nil {
			return installAIAnalysisMsg{pkgKey: key, err: err}
		}
		c.Set(key, text)
		return installAIAnalysisMsg{pkgKey: key, text: text}
	}
}

func installPackageCmdAsync(cmdArgs []string, password string, needsSudo bool) tea.Cmd {
	return func() tea.Msg {
		if needsSudo {
			// Invalidate any cached sudo session so password is authenticated every time
			_ = exec.Command("sudo", "-k").Run()

			// Validate & refresh sudo credentials with sudo -S -v
			vCmd := exec.Command("sudo", "-S", "-v")
			vCmd.Stdin = strings.NewReader(password + "\n")
			if vOut, err := vCmd.CombinedOutput(); err != nil {
				return pkgInstalledMsg{err: fmt.Errorf("incorrect sudo password: %s", strings.TrimSpace(string(vOut))), output: string(vOut)}
			}
		}

		var c *exec.Cmd
		if needsSudo {
			if cmdArgs[0] == "pacman" {
				args := append([]string{"-S"}, cmdArgs...)
				c = exec.Command("sudo", args...)
				c.Stdin = strings.NewReader(password + "\n")
			} else {
				// yay / paru: credentials are now cached in session via sudo -v above
				c = exec.Command(cmdArgs[0], cmdArgs[1:]...)
			}
		} else {
			c = exec.Command(cmdArgs[0], cmdArgs[1:]...)
		}
		out, err := c.CombinedOutput()
		if err != nil {
			return pkgInstalledMsg{err: fmt.Errorf("%v: %s", err, out), output: string(out)}
		}
		return pkgInstalledMsg{err: nil, output: string(out)}
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
		if needsSudo {
			// Invalidate any cached sudo session so password is authenticated every time
			_ = exec.Command("sudo", "-k").Run()

			// Pre-validate credentials with sudo -S -v
			vCmd := exec.Command("sudo", "-S", "-v")
			vCmd.Stdin = strings.NewReader(password + "\n")
			if vOut, err := vCmd.CombinedOutput(); err != nil {
				return pkgRemovedMsg{err: fmt.Errorf("incorrect sudo password: %s", strings.TrimSpace(string(vOut)))}
			}
		}

		var c *exec.Cmd
		if needsSudo {
			args := append([]string{"-S"}, cmdArgs...)
			c = exec.Command("sudo", args...)
			c.Stdin = strings.NewReader(password + "\n")
		} else {
			// Flatpak handles its own permissions
			c = exec.Command(cmdArgs[0], cmdArgs[1:]...)
		}
		out, err := c.CombinedOutput()
		if err != nil {
			return pkgRemovedMsg{err: fmt.Errorf("%v: %s", err, out)}
		}
		return pkgRemovedMsg{err: nil}
	}
}

func waitForSyncProgress(ch <-chan syncProgressMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return syncProgressMsg{DoneAll: true}
		}
		return msg
	}
}

func runSyncWorker(ctx context.Context, aiSvc *ai.Analyzer, c *cache.Cache, pkgs []pm.Package, ch chan<- syncProgressMsg) {
	defer close(ch)

	var explicitPkgs []pm.Package
	var explicitNames []string

	for _, p := range pkgs {
		if p.InstallReason == "Explicitly installed" {
			explicitPkgs = append(explicitPkgs, p)
			explicitNames = append(explicitNames, p.Name)
		}
	}

	total := len(explicitPkgs)
	if total == 0 {
		ch <- syncProgressMsg{Total: 0, Done: 0, DoneAll: true}
		return
	}

	doneCount := 0
	for _, p := range explicitPkgs {
		if c.Has(p.Name, p.Version) {
			doneCount++
		}
	}

	ch <- syncProgressMsg{Total: total, Done: doneCount, DoneAll: doneCount == total}

	if doneCount == total {
		return
	}

	for _, pkg := range explicitPkgs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Re-check cache before fetching
		if c.Has(pkg.Name, pkg.Version) {
			continue
		}

		text, err := aiSvc.Analyze(ctx, &pkg, explicitNames)
		if err == nil {
			key := pkg.Name + "@" + pkg.Version
			c.Set(key, text)
			doneCount++
			ch <- syncProgressMsg{Total: total, Done: doneCount, PkgName: pkg.Name, DoneAll: doneCount == total}
		}
	}
}

func (m Model) resetUpdateModal() Model {
	m.updatingModal = false
	m.updateAskPassword = false
	m.updatingLoading = false
	m.updateErr = ""
	m.updateOutput = ""
	m.updateDone = false
	m.updateTargets = nil
	m.updatePasswordInput.SetValue("")
	m.updatePasswordInput.Blur()
	return m
}

func (m Model) startFullUpgrade() (Model, tea.Cmd) {
	if m.updatingLoading || m.installingLoading || m.removingLoading {
		return m, nil
	}
	m = m.resetUpdateModal()
	m.updatingModal = true
	m.updateTargets = nil // nil = full system/manager upgrade
	activeMgr := m.managers[m.activeMgr]

	const modalW = 76
	const innerW = modalW - 8 - 2
	m.updateOutputVP.Width = innerW
	m.updateOutputVP.Height = 10

	if activeMgr.RequiresSudo() {
		m.updateAskPassword = true
		m.updatePasswordInput.Focus()
		m.updatePasswordInput.SetValue("")
		return m, textinput.Blink
	}

	// Flatpak: run directly without password
	m.updatingLoading = true
	cmdArgs := activeMgr.UpdateCmd()
	return m, updatePackageCmdAsync(cmdArgs, "", false)
}

func (m Model) startUpdate() (Model, tea.Cmd) {
	if m.updatingLoading || m.installingLoading || m.removingLoading {
		return m, nil
	}
	m.commitVisualSelection()
	targets := m.getSelectedNames()
	if len(targets) == 0 {
		return m, nil
	}
	m = m.resetUpdateModal()
	m.updatingModal = true
	m.updateTargets = targets
	activeMgr := m.managers[m.activeMgr]

	const modalW = 76
	const innerW = modalW - 8 - 2
	m.updateOutputVP.Width = innerW
	m.updateOutputVP.Height = 10

	if activeMgr.RequiresSudo() {
		m.updateAskPassword = true
		m.updatePasswordInput.Focus()
		m.updatePasswordInput.SetValue("")
		return m, textinput.Blink
	}

	// Flatpak: run directly without password
	m.updatingLoading = true
	cmdArgs := activeMgr.UpdatePackagesCmd(m.updateTargets)
	return m, updatePackageCmdAsync(cmdArgs, "", false)
}

func (m Model) handleUpdateModalKey(key string, msg tea.KeyMsg) (Model, tea.Cmd) {
	// Phase: password input
	if m.updateAskPassword {
		switch key {
		case "esc":
			m = m.resetUpdateModal()
			return m, nil
		case "enter":
			pw := strings.TrimSpace(m.updatePasswordInput.Value())
			if pw == "" {
				return m, nil
			}
			m.updateAskPassword = false
			m.updatingLoading = true
			m.updatePasswordInput.Blur()
			m.updatePasswordInput.SetValue("")

			activeMgr := m.managers[m.activeMgr]
			var cmdArgs []string
			if len(m.updateTargets) > 0 {
				cmdArgs = activeMgr.UpdatePackagesCmd(m.updateTargets)
			} else {
				cmdArgs = activeMgr.UpdateCmd()
			}
			return m, updatePackageCmdAsync(cmdArgs, pw, true)
		default:
			var cmd tea.Cmd
			m.updatePasswordInput, cmd = m.updatePasswordInput.Update(msg)
			return m, cmd
		}
	}

	// Phase: loading (updating in progress)
	if m.updatingLoading {
		return m, nil
	}

	// Phase: done or error output viewport
	if m.updateDone || m.updateErr != "" {
		switch key {
		case "enter", "esc", "q":
			m = m.resetUpdateModal()
			m.loading = true
			m.selectedPkg = nil
			m.focusedPanel = panelList
			return m, loadPackages(m.managers[m.activeMgr])
		case "j", "down":
			m.updateOutputVP.ScrollDown(1)
		case "k", "up":
			m.updateOutputVP.ScrollUp(1)
		case "ctrl+d":
			m.updateOutputVP.HalfPageDown()
		case "ctrl+u":
			m.updateOutputVP.HalfPageUp()
		case "G":
			m.updateOutputVP.GotoBottom()
		case "g":
			if m.lastKey == "g" {
				m.updateOutputVP.GotoTop()
				m.lastKey = ""
			} else {
				m.lastKey = "g"
			}
		}
		return m, nil
	}

	return m, nil
}

func updatePackageCmdAsync(cmdArgs []string, password string, needsSudo bool) tea.Cmd {
	return func() tea.Msg {
		if needsSudo {
			// Invalidate cached credentials so password is strictly checked
			_ = exec.Command("sudo", "-k").Run()

			vCmd := exec.Command("sudo", "-S", "-v")
			vCmd.Stdin = strings.NewReader(password + "\n")
			if vOut, err := vCmd.CombinedOutput(); err != nil {
				return pkgUpdateOutputMsg{err: fmt.Errorf("incorrect sudo password: %s", strings.TrimSpace(string(vOut))), output: string(vOut)}
			}
		}

		var c *exec.Cmd
		if needsSudo {
			if cmdArgs[0] == "pacman" {
				args := append([]string{"-S"}, cmdArgs...)
				c = exec.Command("sudo", args...)
				c.Stdin = strings.NewReader(password + "\n")
			} else {
				// yay / paru: credentials are cached via sudo -v above
				c = exec.Command(cmdArgs[0], cmdArgs[1:]...)
			}
		} else {
			c = exec.Command(cmdArgs[0], cmdArgs[1:]...)
		}
		out, err := c.CombinedOutput()
		if err != nil {
			return pkgUpdateOutputMsg{err: fmt.Errorf("%v: %s", err, out), output: string(out)}
		}
		return pkgUpdateOutputMsg{err: nil, output: string(out)}
	}
}
