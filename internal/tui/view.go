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

	return mainView
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

func (m Model) renderInstallModal() string {
	mgrName := m.managers[m.activeMgr].Name()
	mgrTitle := strings.ToUpper(mgrName[:1]) + mgrName[1:]

	const modalW = 70              // outer modal width
	const innerW = modalW - 8 - 2 // minus padding(3*2) and borders(1*2) = 60

	var sb strings.Builder
	sb.WriteString(styleTitle.Render("Install Package — "+mgrTitle) + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n\n")

	switch {

	// ── Phase: installing ────────────────────────────────────────────────
	case m.installingLoading:
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Installing "+m.installPkgName+"...") + "\n\n")
		sb.WriteString(styleDimmed.Render("Please wait..."))

	// ── Phase: install error ─────────────────────────────────────────────
	case m.installErr != "":
		sb.WriteString(styleOrphan.Render("Install failed:") + "\n")
		sb.WriteString(styleOrphan.Render(truncate(m.installErr, innerW)) + "\n\n")
		sb.WriteString(styleDimmed.Render("Enter to search again  Esc to close"))

	// ── Phase: password ──────────────────────────────────────────────────
	case m.installAskPassword:
		sb.WriteString(styleVal.Render("Package:  ") + styleTitle.Render(m.installPkgName) + "\n\n")
		sb.WriteString(styleAILabel.Render("sudo password") + "\n")
		sb.WriteString(m.installPasswordInput.View() + "\n\n")
		sb.WriteString(styleDimmed.Render("Enter to install  Esc to go back"))

	// ── Phase: package description popup ────────────────────────────────
	case m.installShowDesc && len(m.installResults) > 0:
		pkg := m.installResults[m.installResultsCursor]
		sb.WriteString(styleTitle.Render(pkg.Name) + "  " + styleDimmed.Render(pkg.Version) + "\n")
		sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n\n")

		desc := pkg.Description
		if desc == "" {
			desc = "No description available."
		}
		sb.WriteString(styleVal.Render(wrapText(desc, innerW)) + "\n\n")

		sb.WriteString(styleDimmed.Render("Tab/Esc back to list  Enter select & install"))

	// ── Phase: results list ──────────────────────────────────────────────
	case len(m.installResults) > 0:
		count := len(m.installResults)
		query := truncate(m.installPkgInput.Value(), 24)
		sb.WriteString(styleAILabel.Render(fmt.Sprintf("%d results", count)) +
			styleDimmed.Render("  for: "+query) + "\n")
		sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")

		const listH = 8
		start := m.installResultsOffset
		end := min(start+listH, count)

		for i := start; i < end; i++ {
			pkg := m.installResults[i]
			hovered := i == m.installResultsCursor

			// Layout: name(24) + " " + version(10) + "  " + desc(rest)
			const nameW, verW = 24, 10
			descW := innerW - nameW - verW - 3 // 3 for separating spaces
			name := padRight(truncate(pkg.Name, nameW), nameW)
			ver := padRight(truncate(pkg.Version, verW), verW)
			desc := truncate(pkg.Description, descW)
			content := name + " " + ver + "  " + desc

			if hovered {
				sb.WriteString(styleSelected.
					Background(colorBorderFoc).
					Foreground(colorBase).
					Render(" › "+truncate(content, innerW-2)) + "\n")
			} else {
				sb.WriteString(styleDimmed.Render("   "+truncate(content, innerW-3)) + "\n")
			}
		}

		// Scroll indicator
		if count > listH {
			pct := int(float64(m.installResultsCursor) / float64(count-1) * 100)
			sb.WriteString(styleDimmed.Render(
				fmt.Sprintf("\n  %d/%d  %d%%", m.installResultsCursor+1, count, pct)))
		}

		sb.WriteString("\n\n" + styleDimmed.Render("j/k navigate  ") +
			styleKey.Render("Tab") + styleDimmed.Render(" desc  Enter select  Esc/") +
			styleKey.Render("/") + styleDimmed.Render(" search"))

	// ── Phase: searching ─────────────────────────────────────────────────
	case m.installSearching:
		sb.WriteString(styleDimmed.Render("Query: ")+styleVal.Render(m.installPkgInput.Value()) + "\n\n")
		sb.WriteString(m.spinner.View() + " " + styleDimmed.Render("Searching...") + "\n\n")
		sb.WriteString(styleDimmed.Render("Esc to cancel"))

	// ── Phase: search error ──────────────────────────────────────────────
	case m.installSearchErr != "":
		sb.WriteString(styleOrphan.Render(m.installSearchErr) + "\n\n")
		sb.WriteString(styleDimmed.Render("Enter to retry  Esc to go back"))

	// ── Phase: search input (default) ────────────────────────────────────
	default:
		sb.WriteString(styleDimmed.Render("Search for a package to install:") + "\n\n")
		sb.WriteString(m.installPkgInput.View() + "\n\n")
		sb.WriteString(styleDimmed.Render("Enter to search  Esc to close"))
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
	if m.focusedPanel != panelDetail {
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
	} else if m.focusedPanel == panelDetail {
		hints = []string{
			styleKey.Render("x") + " remove",
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
			styleKey.Render("s") + " sort",
			styleKey.Render("i") + " install",
			styleKey.Render("o") + " orphans",
			styleKey.Render("/") + " search",
			styleKey.Render("q") + " quit",
		}
	}

	bar := styleStatusBar.Render(strings.Join(hints, "  "))
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
