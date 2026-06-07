package tui

import (
	"fmt"
	"strings"

	"orpheus/internal/pm"
	"orpheus/internal/svc"

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

	return lipgloss.JoinVertical(lipgloss.Left, content, status)
}

func (m Model) renderSidebar() string {
	w := m.sidebarWidth()
	h := m.height - 3

	var sb strings.Builder
	sb.WriteString(styleTitle.Render("  Orpheus") + "\n\n")

	views := []struct {
		id    viewID
		icon  string
		label string
	}{
		{viewPackages, "  ", "Packages"},
		{viewCleanup, "  ", "Cleanup"},
		{viewServices, "  ", "Services"},
	}

	for _, v := range views {
		line := v.icon + v.label
		if m.activeView == v.id {
			line = styleSidebarActive.Render("> " + v.icon + v.label)
		} else {
			line = styleDimmed.Render("  " + v.icon + v.label)
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

	switch m.activeView {
	case viewPackages:
		sb.WriteString(m.renderPackageList(w, inner))
	case viewCleanup:
		sb.WriteString(m.renderCleanupList(w, inner))
	case viewServices:
		sb.WriteString(m.renderServicesList(w, inner))
	}

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
	title := "Packages (" + count + ")"
	if m.searching || m.searchInput.Value() != "" {
		title = "Packages  " + styleAILabel.Render("/"+m.searchInput.Value())
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
		line := renderPkgLine(p, w-4, i == m.listCursor)
		sb.WriteString(line + "\n")
	}

	// scroll indicator
	if len(pkgs) > visible {
		pct := int(float64(m.listCursor) / float64(len(pkgs)-1) * 100)
		sb.WriteString(styleDimmed.Render(fmt.Sprintf("\n  %d/%d  %d%%", m.listCursor+1, len(pkgs), pct)))
	}

	return sb.String()
}

func renderPkgLine(p pm.Package, width int, selected bool) string {
	badge := badgeFor(p)
	nameWidth := width - 12
	name := truncate(p.Name, nameWidth)

	var reason string
	switch p.ReasonShort() {
	case "ORPHAN":
		reason = styleOrphan.Render("ORPHAN")
	case "EXPLICIT":
		reason = styleExplicit.Render("EXPLIC")
	default:
		reason = styleDimmed.Render("DEP   ")
	}

	line := badge + " " + padRight(name, nameWidth) + " " + reason

	if selected {
		return styleSelected.Render(" " + line + " ")
	}
	return "  " + line
}

func badgeFor(p pm.Package) string {
	if p.IsOrphan {
		return styleOrphan.Render("●")
	}
	if p.InstallReason == "Explicitly installed" {
		return styleExplicit.Render("●")
	}
	return styleDimmed.Render("●")
}

func (m Model) renderCleanupList(w, h int) string {
	var sb strings.Builder
	sb.WriteString(styleTitle.Render("Cleanup Queue") + "  " + styleDimmed.Render("ranked by size") + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", w-2)) + "\n")

	if !m.cleanupLoaded {
		sb.WriteString("\n  " + m.spinner.View() + " Scanning for orphans...\n")
		return sb.String()
	}

	if len(m.cleanupPkgs) == 0 {
		sb.WriteString("\n  " + styleHealthLow.Render("✓ No orphaned packages found") + "\n")
		sb.WriteString("  " + styleDimmed.Render("Your system is clean.") + "\n")
		return sb.String()
	}

	visible := h - 2
	for i, p := range m.cleanupPkgs {
		if i >= visible {
			break
		}
		line := renderCleanupLine(p, w-4, i == m.cleanupCursor)
		sb.WriteString(line + "\n")
	}

	return sb.String()
}

func renderCleanupLine(p pm.Package, width int, selected bool) string {
	badge := p.HealthBadge()
	size := fmt.Sprintf("%-10s", p.FormatSize())
	nameWidth := width - len(badge) - len(size) - 4
	name := truncate(p.Name, nameWidth)

	line := badge + " " + padRight(name, nameWidth) + " " + styleDimmed.Render(size)

	if selected {
		return styleSelected.Render(" " + line + " ")
	}
	return "  " + line
}

func (m Model) renderServicesList(w, h int) string {
	var sb strings.Builder
	sb.WriteString(styleTitle.Render("Services") + "  " + styleDimmed.Render("systemd units") + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", w-2)) + "\n")

	if !m.svcLoaded {
		sb.WriteString("\n  " + m.spinner.View() + " Loading services...\n")
		return sb.String()
	}

	if len(m.services) == 0 {
		sb.WriteString("\n  " + styleDimmed.Render("No services found") + "\n")
		return sb.String()
	}

	visible := h - 2
	for i, s := range m.services {
		if i >= visible {
			break
		}
		line := renderServiceLine(s, w-4, i == m.svcCursor)
		sb.WriteString(line + "\n")
	}

	return sb.String()
}

func renderServiceLine(s svc.Service, width int, selected bool) string {
	badge := s.StatusBadge()
	var badgeStyled string
	switch {
	case s.IsActive():
		badgeStyled = styleHealthLow.Render(badge)
	case s.IsFailed():
		badgeStyled = styleHealthHigh.Render(badge)
	default:
		badgeStyled = styleDimmed.Render(badge)
	}

	nameWidth := width - 14
	name := truncate(strings.TrimSuffix(s.Unit, ".service"), nameWidth)
	owner := ""
	if s.OwnerPkg != "" {
		owner = styleDimmed.Render(truncate(s.OwnerPkg, 12))
	} else {
		owner = styleDimmed.Render("unknown     ")
	}

	line := badgeStyled + " " + padRight(name, nameWidth) + " " + owner

	if selected {
		return styleSelected.Render(" " + line + " ")
	}
	return "  " + line
}

func (m Model) renderDetailPanel() string {
	w := m.detailWidth()
	h := m.height - 3

	var content string
	if m.selectedPkg == nil || m.focusedPanel != panelDetail {
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
	switch m.activeView {
	case viewPackages:
		if m.searching {
			hints = []string{
				styleKey.Render("Enter") + " confirm",
				styleKey.Render("Esc") + " cancel",
			}
		} else if m.focusedPanel == panelDetail {
			hints = []string{
				styleKey.Render("a") + " analyze",
				styleKey.Render("j/k") + " scroll",
				styleKey.Render("h/Esc") + " back",
				styleKey.Render("q") + " quit",
			}
		} else {
			hints = []string{
				styleKey.Render("j/k") + " move",
				styleKey.Render("l/Enter") + " open",
				styleKey.Render("/") + " search",
				styleKey.Render("1-3") + " views",
				styleKey.Render("q") + " quit",
			}
		}
	case viewCleanup:
		hints = []string{
			styleKey.Render("j/k") + " move",
			styleKey.Render("Enter") + " detail",
			styleKey.Render("1-3") + " views",
			styleKey.Render("q") + " quit",
		}
	case viewServices:
		hints = []string{
			styleKey.Render("j/k") + " move",
			styleKey.Render("1-3") + " views",
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
