package pm

import (
	"strconv"
	"strings"
)

type Flatpak struct{}

func NewFlatpak() *Flatpak { return &Flatpak{} }

func (p *Flatpak) Name() string { return "flatpak" }

func (p *Flatpak) UninstallCmd(names []string) []string {
	// Chain both the app removal (with data deletion) and the unused runtime cleanup.
	// Use sh -c so we can run two commands sequentially.
	cmdStr := "dbus-run-session flatpak uninstall -y --delete-data " + strings.Join(names, " ") + " && dbus-run-session flatpak uninstall -y --unused"
	return []string{"sh", "-c", cmdStr}
}

func (p *Flatpak) UninstallOrphansCmd() []string {
	cmdStr := "dbus-run-session flatpak uninstall -y --unused"
	return []string{"sh", "-c", cmdStr}
}

func (p *Flatpak) GetOrphans() ([]string, error) {
	out, err := runCmdAllowExit1("flatpak", "uninstall", "--unused")
	if err != nil {
		return []string{}, nil
	}
	str := string(out)
	if strings.Contains(str, "Nothing unused") || strings.TrimSpace(str) == "" {
		return []string{}, nil
	}
	lines := strings.Split(str, "\n")
	var orphans []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "Nothing unused") {
			orphans = append(orphans, l)
		}
	}
	return orphans, nil
}

func (p *Flatpak) ListAll() ([]Package, error) {
	// flatpak list outputs tab-separated columns when piped.
	// Columns: application, name, version, size, description, arch
	// :f prevents ellipsization
	out, err := runCmd("flatpak", "list", "--app", "--columns=application:f,name:f,version:f,size:f,description:f,arch:f")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	var pkgs []Package

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 6 {
			continue // skip invalid lines
		}

		appID := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		version := strings.TrimSpace(parts[2])
		sizeStr := strings.TrimSpace(parts[3])
		desc := strings.TrimSpace(parts[4])
		arch := strings.TrimSpace(parts[5])

		// For Orpheus UI, the main identifier is Name. Let's use the appID as Name
		// because that's what flatpak uninstall requires. We could use Name for the real name,
		// but since we uninstall by name, we need the app ID as the Package.Name.
		pkg := Package{
			Name:          appID,
			Version:       version,
			Description:   name + " - " + desc, // Prepend real name to description
			Architecture:  arch,
			Size:          parseFlatpakSize(sizeStr),
			InstallReason: "Explicitly installed", // Assume apps are explicitly installed
		}
		pkgs = append(pkgs, pkg)
	}

	return pkgs, nil
}

func (p *Flatpak) GetPackage(name string) (*Package, error) {
	// The TUI only calls this for single details but we already get most details from ListAll.
	// Since Orpheus actually calls ListAll once and filters in memory, this is rarely needed
	// for full fetch in pacman, except when getting detailed dependencies.
	// We can try to use flatpak info, or just return basic info.
	// Let's implement a dummy fallback. In tui, GetPackage isn't even used to fetch data for the list!
	// It's mainly used for pacman detailed deps. Actually, let's see if TUI calls it.
	return nil, nil // Return nil, nil if unsupported or implement flatpak info.
}

func parseFlatpakSize(s string) int64 {
	s = strings.ReplaceAll(s, "\u00A0", " ") // non-breaking space
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	parts := strings.Split(s, " ")
	if len(parts) != 2 {
		return 0
	}
	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	unit := strings.ToLower(parts[1])
	var mult float64 = 1
	switch {
	case strings.HasPrefix(unit, "kb"):
		mult = 1024
	case strings.HasPrefix(unit, "mb"):
		mult = 1024 * 1024
	case strings.HasPrefix(unit, "gb"):
		mult = 1024 * 1024 * 1024
	case strings.HasPrefix(unit, "byte"):
		mult = 1
	}
	return int64(num * mult)
}
