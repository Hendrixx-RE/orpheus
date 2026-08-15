package tui

import (
	"fmt"
	"strings"

	"orpheus/internal/pm"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if !m.ready {
		return "\n  Loading Orpheus...\n"
	}

	sidebar := m.renderSidebar()
	list := m.renderListPanel()
	detail := m.renderDetailPanel()

	content := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, list, detail)
	status := m.renderStatusBar()

	mainView := lipgloss.JoinVertical(lipgloss.Left, content, status)

	if m.removingOrphans {
		return m.renderOrphanModal()
	}

	if m.installingModal {
		return m.renderInstallModal()
	}

	if m.updatingModal {
		return m.renderUpdateModal()
	}

	return mainView
}

func (m Model) renderUpdateModal() string {
	mgrName := m.managers[m.activeMgr].Name()
	mgrTitle := strings.ToUpper(mgrName[:1]) + mgrName[1:]
	if mgrName == "pacman" {
		mgrTitle = "Pacman / AUR"
	}

	const modalW = 76              // outer modal width
	const innerW = modalW - 8 - 2 // minus padding(3*2) and borders(1*2) = 66

	var sb strings.Builder
	title := "Update Packages"
	if len(m.updateTargets) > 0 {
		title = fmt.Sprintf("Update %d Package(s)", len(m.updateTargets))
	} else {
		title = "System Upgrade — " + mgrTitle
	}
	sb.WriteString(styleTitle.Render(title) + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n\n")

	switch {
	// Phase: password input
	case m.updateAskPassword:
		if len(m.updateTargets) > 0 {
			sb.WriteString(styleVal.Render(fmt.Sprintf("Ready to update %d package(s) via %s.", len(m.updateTargets), mgrTitle)) + "\n\n")
		} else {
			sb.WriteString(styleVal.Render(fmt.Sprintf("Ready to upgrade all system packages via %s.", mgrTitle)) + "\n\n")
		}
		sb.WriteString(styleAILabel.Render("sudo password") + "\n")
		sb.WriteString(m.updatePasswordInput.View() + "\n\n")
		sb.WriteString(styleDimmed.Render("Enter to start update  |  Esc to cancel"))

	// Phase: updating in progress
	case m.updatingLoading:
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Updating packages via "+mgrTitle+"...") + "\n\n")
		sb.WriteString(styleDimmed.Render("Please wait, resolving dependencies and applying updates..."))

	// Phase: update error
	case m.updateErr != "":
		sb.WriteString(styleOrphan.Render("Update failed:") + "\n")
		if m.updateOutput != "" {
			sb.WriteString(styleVal.Render(truncate(m.updateErr, innerW)) + "\n\n")
			sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")
			sb.WriteString(m.updateOutputVP.View() + "\n")
			sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n\n")
		} else {
			sb.WriteString(styleOrphan.Render(wrapText(m.updateErr, innerW)) + "\n\n")
		}
		sb.WriteString(styleDimmed.Render("j/k scroll log  |  Enter/Esc to close and reload"))

	// Phase: update complete (done)
	case m.updateDone:
		sb.WriteString(styleVerdict.Render("✓ Update complete!") + "\n\n")
		if m.updateOutput != "" {
			sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")
			sb.WriteString(m.updateOutputVP.View() + "\n")
			sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n\n")
		}
		sb.WriteString(styleDimmed.Render("j/k scroll log  |  Enter/Esc to close and reload"))

	default:
		sb.WriteString(styleDimmed.Render("Ready to update.") + "\n\n")
		sb.WriteString(styleDimmed.Render("Enter to confirm  |  Esc to close"))
	}

	modalBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorCyan).
		Padding(1, 3).
		Width(modalW).
		Render(sb.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalBox)
}

func (m Model) renderOrphanModal() string {
	mgrName := m.managers[m.activeMgr].Name()
	mgrTitle := strings.ToUpper(mgrName[:1]) + mgrName[1:]
	var sb strings.Builder

	sb.WriteString(styleTitle.Render("Orphan Cleanup — "+mgrTitle) + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", 44)) + "\n\n")

	switch {
	case m.checkingOrphans:
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Checking for orphan packages...") + "\n\n")
		sb.WriteString(styleDimmed.Render("Please wait..."))
	case m.askingPassword:
		count := len(m.orphanList)
		sb.WriteString(styleVal.Render(fmt.Sprintf("Found %d orphan package(s).", count)) + "\n")
		sb.WriteString(styleVal.Render("Enter sudo password to remove:") + "\n\n")
		sb.WriteString(m.passwordInput.View() + "\n\n")
		sb.WriteString(styleDimmed.Render("(Press Esc to cancel)"))
	case m.removingLoading:
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Removing orphan packages...") + "\n\n")
		sb.WriteString(styleDimmed.Render("Please wait..."))
	case m.removeErr != "":
		sb.WriteString(styleOrphan.Render("Removal failed:") + "\n")
		sb.WriteString(styleOrphan.Render(truncate(m.removeErr, 44)) + "\n\n")
		sb.WriteString(styleDimmed.Render("Press o to retry, or Esc to close"))
	case len(m.orphanList) == 0:
		sb.WriteString(styleVal.Render("No orphans to clean.") + "\n\n")
		sb.WriteString(styleDimmed.Render("(Press Esc to close)"))
	default:
		sb.WriteString(styleDimmed.Render("Press o to remove orphan packages for "+mgrName) + "\n\n")
		sb.WriteString(styleDimmed.Render("(Press Esc to close)"))
	}

	modalBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFoc).
		Padding(1, 3).
		Width(50).
		Render(sb.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalBox)
}

func renderRepoTag(repo string) string {
	switch strings.ToLower(repo) {
	case "aur":
		return lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render("[aur]")
	case "flathub":
		return lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render("[flathub]")
	case "core", "extra", "multilib":
		return lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("[" + repo + "]")
	default:
		if repo == "" {
			return ""
		}
		return styleDimmed.Render("[" + repo + "]")
	}
}

func (m Model) renderInstallModal() string {
	mgrIdx := m.installMgrIndex
	if mgrIdx < 0 || mgrIdx >= len(m.managers) {
		mgrIdx = m.activeMgr
	}
	mgrName := m.managers[mgrIdx].Name()
	mgrTitle := strings.ToUpper(mgrName[:1]) + mgrName[1:]
	if mgrName == "pacman" {
		mgrTitle = "Pacman / AUR"
	}

	const modalW = 76              // outer modal width
	const innerW = modalW - 8 - 2 // minus padding(3*2) and borders(1*2) = 66

	var sb strings.Builder
	sb.WriteString(styleTitle.Render("Install Package") + styleDimmed.Render(" — ") + styleAILabel.Render(mgrTitle) + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n\n")

	switch {

	// ── Phase: installing ────────────────────────────────────────────────
	case m.installingLoading:
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Installing "+m.installPkgName+"...") + "\n\n")
		sb.WriteString(styleDimmed.Render("Please wait, resolving dependencies and installing..."))

	// ── Phase: install error ─────────────────────────────────────────────
	case m.installErr != "":
		sb.WriteString(styleOrphan.Render("Install failed:") + "\n")
		if m.installOutput != "" {
			sb.WriteString(styleVal.Render(truncate(m.installErr, innerW)) + "\n\n")
			sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")
			sb.WriteString(m.installOutputVP.View() + "\n")
			sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n\n")
		} else {
			sb.WriteString(styleOrphan.Render(wrapText(m.installErr, innerW)) + "\n\n")
		}
		sb.WriteString(styleDimmed.Render("Enter to search again  |  Esc to close"))

	// ── Phase: password ──────────────────────────────────────────────────
	case m.installAskPassword:
		sb.WriteString(styleVal.Render("Package:  ") + styleTitle.Render(m.installPkgName) + "\n\n")
		sb.WriteString(styleAILabel.Render("sudo password") + "\n")
		sb.WriteString(m.installPasswordInput.View() + "\n\n")
		sb.WriteString(styleDimmed.Render("Enter to confirm install  |  Esc to go back"))

	// ── Phase: package preview & AI analysis ─────────────────────────────
	case m.installShowDesc && len(m.installResults) > 0:
		pkg := m.installResults[m.installResultsCursor]
		tag := renderRepoTag(pkg.Repository)
		instBadge := ""
		if pkg.IsInstalled {
			instBadge = "  " + styleVerdict.Render("[installed]")
		}

		titleLine := styleTitle.Render(pkg.Name) + "  " + styleDimmed.Render(pkg.Version)
		if tag != "" {
			titleLine += "  " + tag
		}
		titleLine += instBadge
		sb.WriteString(titleLine + "\n")
		sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n\n")

		desc := pkg.Description
		if desc == "" {
			desc = "No description available."
		}
		sb.WriteString(styleVal.Render(wrapText(desc, innerW)) + "\n\n")

		sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")
		sb.WriteString(styleAILabel.Render("AI Short Description") + "\n")
		sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n\n")

		switch {
		case m.installAILoading:
			sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Fetching short description...") + "\n\n")
		case m.installAIErr != "":
			sb.WriteString(styleOrphan.Render(m.installAIErr) + "\n")
			sb.WriteString(styleDimmed.Render("Press a to retry") + "\n\n")
		case m.installAIAnalysis != "":
			sb.WriteString(styleVal.Render(wrapText(strings.TrimSpace(m.installAIAnalysis), innerW)) + "\n\n")
		default:
			sb.WriteString(styleDimmed.Render("Press ") + styleKey.Render("a") + styleDimmed.Render(" for an AI short description.") + "\n\n")
		}

		sb.WriteString(styleDimmed.Render("Tab/Esc back to list  |  ") + styleKey.Render("a") + styleDimmed.Render(" AI summary  |  Enter install"))

	// ── Phase: results list ──────────────────────────────────────────────
	case len(m.installResults) > 0:
		count := len(m.installResults)
		query := truncate(m.installPkgInput.Value(), 24)
		sb.WriteString(styleAILabel.Render(fmt.Sprintf("%d results", count)) +
			styleDimmed.Render(" for \""+query+"\" in "+mgrTitle) + "\n")
		sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")

		const listH = 8
		start := m.installResultsOffset
		end := min(start+listH, count)

		for i := start; i < end; i++ {
			pkg := m.installResults[i]
			hovered := i == m.installResultsCursor

			tag := renderRepoTag(pkg.Repository)
			tagLen := 0
			if pkg.Repository != "" {
				tagLen = len(pkg.Repository) + 3 // "[repo] "
			}

			instBadge := ""
			instLen := 0
			if pkg.IsInstalled {
				instBadge = lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("✓")
				instLen = 2 // "✓ "
			}

			const nameW, verW = 20, 10
			nameStr := padRight(truncate(pkg.Name, nameW), nameW)
			verStr := padRight(truncate(pkg.Version, verW), verW)

			descW := innerW - nameW - verW - tagLen - instLen - 7
			descStr := truncate(pkg.Description, max(8, descW))

			var nameRendered string
			if hovered {
				nameRendered = lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render(nameStr)
			} else {
				nameRendered = lipgloss.NewStyle().Foreground(colorText).Render(nameStr)
			}

			prefix := "   "
			if hovered {
				prefix = lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render(" › ")
			}

			row := prefix
			if tag != "" {
				row += tag + " "
			}
			row += nameRendered + " " + styleDimmed.Render(verStr) + " "
			if instBadge != "" {
				row += instBadge + " "
			}
			row += styleDimmed.Render(descStr)

			sb.WriteString(row + "\n")
		}

		// Scroll indicator
		if count > listH {
			pct := int(float64(m.installResultsCursor) / float64(count-1) * 100)
			sb.WriteString(styleDimmed.Render(
				fmt.Sprintf("\n  %d/%d  %d%%", m.installResultsCursor+1, count, pct)))
		}

		sb.WriteString("\n\n" + styleDimmed.Render("j/k navigate  |  ") +
			styleKey.Render("Tab") + styleDimmed.Render(" inspect & AI  |  Enter install  |  Esc/") +
			styleKey.Render("/") + styleDimmed.Render(" search"))

	// ── Phase: searching ─────────────────────────────────────────────────
	case m.installSearching:
		sb.WriteString(styleDimmed.Render("Searching ")+styleAILabel.Render(mgrTitle)+styleDimmed.Render(" for: ")+styleVal.Render(m.installPkgInput.Value()) + "\n\n")
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Searching repositories...") + "\n\n")
		sb.WriteString(styleDimmed.Render("Esc to cancel"))

	// ── Phase: search error ──────────────────────────────────────────────
	case m.installSearchErr != "":
		sb.WriteString(styleOrphan.Render(m.installSearchErr) + "\n\n")
		sb.WriteString(styleDimmed.Render("Tab switch manager  |  Enter to retry  |  Esc to go back"))

	// ── Phase: search input (default) ────────────────────────────────────
	default:
		sb.WriteString(styleDimmed.Render("Search packages to install via ") + styleAILabel.Render(mgrTitle) + styleDimmed.Render(":") + "\n\n")
		sb.WriteString(m.installPkgInput.View() + "\n\n")
		sb.WriteString(styleDimmed.Render("Tab switch manager  |  Enter to search  |  Esc to close"))
	}

	modalBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorCyan).
		Padding(1, 3).
		Width(modalW).
		Render(sb.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalBox)
}


func (m Model) renderSidebar() string {
	w := m.sidebarWidth()
	h := m.height - 3

	var sb strings.Builder
	sb.WriteString(styleTitle.Render("  Orpheus") + "\n\n")
	sb.WriteString(styleTitle.Render("  Packages") + "\n")

	for i, mgr := range m.managers {
		var line string
		label := mgr.Name()
		if len(label) > 0 {
			label = strings.ToUpper(label[:1]) + label[1:]
		}
		
		if m.activeMgr == i {
			line = styleSidebarActive.Render(">   " + label)
		} else {
			line = styleDimmed.Render("    " + label)
		}
		sb.WriteString(line + "\n")
	}

	// fill remaining height
	lines := strings.Count(sb.String(), "\n")
	for i := lines; i < h-2; i++ {
		sb.WriteString("\n")
	}

	box := sb.String()
	st := stylePanel
	if m.focusedPanel == panelSidebar {
		st = stylePanelFocused
	}
	return st.Width(w).Height(h).Render(box)
}

func (m Model) renderListPanel() string {
	w := m.listWidth()
	h := m.height - 3
	inner := h - 2 // minus borders
	var sb strings.Builder
	sb.WriteString(m.renderPackageList(w, inner))

	st := stylePanel
	if m.focusedPanel == panelList {
		st = stylePanelFocused
	}
	return st.Width(w).Height(h).Render(sb.String())
}

func (m Model) renderPackageList(w, h int) string {
	var sb strings.Builder

	// header
	count := fmt.Sprintf("%d", len(m.filteredPkgs))

	var sortLabel string
	switch m.sortMode {
	case sortByName:
		sortLabel = "Name"
	case sortBySize:
		sortLabel = "Size"
	case sortByDate:
		sortLabel = "Date"
	}
	title := "Packages (" + count + ")  " + styleDimmed.Render("by "+sortLabel)

	if m.searching || m.searchInput.Value() != "" {
		title = "Packages  " + styleAILabel.Render("/"+m.searchInput.Value()) + "  " + styleDimmed.Render("by "+sortLabel)
	}

	sb.WriteString(styleTitle.Render(title) + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", w-2)) + "\n")

	if m.loading {
		sb.WriteString("\n  " + m.spinner.View() + " Loading packages...\n")
		return sb.String()
	}

	pkgs := m.filteredPkgs
	if len(pkgs) == 0 {
		sb.WriteString("\n  " + styleDimmed.Render("No packages found") + "\n")
		return sb.String()
	}

	visible := h - 2 // header + divider
	start := m.listOffset
	end := min(start+visible, len(pkgs))

	for i := start; i < end; i++ {
		p := pkgs[i]
		checked := m.isPkgSelected(i)
		hovered := i == m.listCursor
		line := renderPkgLine(p, w-4, checked, hovered)
		sb.WriteString(line + "\n")
	}

	// scroll indicator
	if len(pkgs) > visible {
		pct := int(float64(m.listCursor) / float64(len(pkgs)-1) * 100)
		sb.WriteString(styleDimmed.Render(fmt.Sprintf("\n  %d/%d  %d%%", m.listCursor+1, len(pkgs), pct)))
	}

	return sb.String()
}

func renderPkgLine(p pm.Package, width int, checked bool, hovered bool) string {
	size := fmt.Sprintf("%-10s", p.FormatSize())
	nameWidth := width - 16
	name := truncate(p.Name, nameWidth)

	var badge string
	if checked {
		badge = lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render("✓")
	} else {
		badge = styleExplicit.Render("●")
	}

	line := badge + " " + padRight(name, nameWidth) + " " + styleDimmed.Render(size)

	if hovered {
		hoverBadge := lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render("●")
		if checked {
			hoverBadge = lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render("✓")
		}
		row := hoverBadge + " " + styleSelected.Background(lipgloss.NoColor{}).Foreground(colorYellow).Render(padRight(name, nameWidth)) + " " + styleDimmed.Render(size)
		return "  " + row
	}

	return "  " + line
}



func (m Model) renderDetailPanel() string {
	w := m.detailWidth()
	h := m.height - 3

	var content string
	if m.askingPassword || m.removingLoading || m.removeErr != "" {
		// Render the action view (password/spinner/error) directly in the
		// panel instead of the viewport so it never overflows.
		content = m.renderActionView(w)
	} else if m.focusedPanel != panelDetail {
		content = m.renderDetailEmpty()
	} else if m.selectedPkg == nil && len(m.selectedPkgs) <= 1 {
		content = m.renderDetailEmpty()
	} else {
		content = m.detailVP.View()
	}

	st := stylePanel
	if m.focusedPanel == panelDetail {
		st = stylePanelFocused
	}
	return st.Width(w).Height(h).Padding(0, 2).Render(content)
}

// renderActionView builds the full detail-panel content shown during
// password prompt, removal spinner, or removal error states.
func (m Model) renderActionView(w int) string {
	divider := styleDivider.Render(strings.Repeat("─", w-6))
	var sb strings.Builder

	// Show package / batch title
	if len(m.selectedPkgs) > 1 {
		sb.WriteString(styleTitle.Render("Batch Operation") + "\n")
	} else if m.selectedPkg != nil {
		sb.WriteString(styleTitle.Render(m.selectedPkg.Name) + "  " + styleDimmed.Render(m.selectedPkg.Version) + "\n")
	}
	sb.WriteString(divider + "\n\n")

	// Action state
	if m.askingPassword {
		if len(m.selectedPkgs) > 1 {
			names := m.getSelectedNames()
			sb.WriteString(styleOrphan.Render(fmt.Sprintf("Remove %d Packages", len(names))) + "\n\n")
		}
		sb.WriteString(styleAILabel.Render("sudo password") + "\n")
		sb.WriteString(m.passwordInput.View() + "\n\n")
		sb.WriteString(styleDimmed.Render("Enter to confirm  Esc to cancel") + "\n")
	} else if m.removingLoading {
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Removing package...") + "\n")
	} else if m.removeErr != "" {
		sb.WriteString(styleOrphan.Render("Removal failed:") + "\n")
		sb.WriteString(styleOrphan.Render(wrapText(m.removeErr, w-8)) + "\n\n")
		sb.WriteString(styleDimmed.Render("Press x to retry") + "\n")
	}

	return sb.String()
}

func (m Model) renderDetailEmpty() string {
	var sb strings.Builder
	sb.WriteString(styleTitle.Render("Detail") + "\n\n")
	sb.WriteString(styleDimmed.Render("Select a package with Enter\n"))
	return sb.String()
}

func (m Model) renderStatusBar() string {
	var hints []string
	if m.searching {
		hints = []string{
			styleKey.Render("Enter") + " confirm",
			styleKey.Render("Esc") + " cancel",
		}
	} else if m.focusedPanel == panelSidebar {
		hints = []string{
			styleKey.Render("j/k") + " move",
			styleKey.Render("l/Enter") + " list",
			styleKey.Render("u/U") + " upgrade",
			styleKey.Render("i") + " install",
			styleKey.Render("o") + " orphans",
			styleKey.Render("q") + " quit",
		}
	} else if m.focusedPanel == panelDetail {
		hints = []string{
			styleKey.Render("x") + " remove",
			styleKey.Render("u") + " update",
			styleKey.Render("U") + " upgrade all",
			styleKey.Render("i") + " install",
			styleKey.Render("o") + " orphans",
			styleKey.Render("j/k") + " scroll",
			styleKey.Render("h/Esc") + " back",
			styleKey.Render("q") + " quit",
		}
	} else {
		hints = []string{
			styleKey.Render("v/Spc") + " select",
			styleKey.Render("j/k") + " move",
			styleKey.Render("l/Enter") + " open",
			styleKey.Render("u") + " update",
			styleKey.Render("U") + " upgrade all",
			styleKey.Render("s") + " sort",
			styleKey.Render("i") + " install",
			styleKey.Render("o") + " orphans",
			styleKey.Render("/") + " search",
			styleKey.Render("q") + " quit",
		}
	}

	left := strings.Join(hints, "  ")

	var syncIndicator string
	if m.syncTotal > 0 {
		if m.syncActive {
			syncIndicator = styleAILabel.Render(fmt.Sprintf("[AI %d/%d]", m.syncDone, m.syncTotal))
		} else if m.syncDone == m.syncTotal {
			syncIndicator = styleVerdict.Render("[AI 100%]")
		} else {
			syncIndicator = styleDimmed.Render(fmt.Sprintf("[AI %d/%d]", m.syncDone, m.syncTotal))
		}
	}

	if syncIndicator != "" {
		totalW := m.width - 4
		leftW := lipgloss.Width(left)
		rightW := lipgloss.Width(syncIndicator)
		spaceW := totalW - leftW - rightW
		if spaceW > 2 {
			bar := styleStatusBar.Render(left) + strings.Repeat(" ", spaceW) + syncIndicator
			return "  " + bar
		}
		left = left + "  " + syncIndicator
	}

	bar := styleStatusBar.Render(left)
	return "  " + bar
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
