package pm

import (
	"bufio"
	"bytes"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Pacman struct{}

func NewPacman() *Pacman { return &Pacman{} }

func (p *Pacman) Name() string { return "pacman" }

func getPacmanHelper() string {
	if _, err := exec.LookPath("yay"); err == nil {
		return "yay"
	}
	if _, err := exec.LookPath("paru"); err == nil {
		return "paru"
	}
	return "pacman"
}

func (p *Pacman) RequiresSudo() bool {
	return true
}

func (p *Pacman) UninstallCmd(names []string) []string {
	return append([]string{"pacman", "-Rns", "--noconfirm"}, names...)
}

func (p *Pacman) InstallCmd(name string) []string {
	return []string{"pacman", "-S", "--noconfirm", name}
}

func (p *Pacman) UpdateCmd() []string {
	return []string{"pacman", "-Syu", "--noconfirm"}
}

func (p *Pacman) UpdatePackagesCmd(names []string) []string {
	return append([]string{"pacman", "-S", "--noconfirm"}, names...)
}

func (p *Pacman) CleanCacheCmd() []string {
	return []string{"pacman", "-Sc", "--noconfirm"}
}

func parseCheckupdatesLine(line string, mgrName string) (UpdatablePackage, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return UpdatablePackage{}, false
	}
	parts := strings.Fields(line)
	if len(parts) >= 4 && parts[len(parts)-2] == "->" {
		return UpdatablePackage{
			Name:       parts[0],
			OldVersion: parts[1],
			NewVersion: parts[3],
			Manager:    mgrName,
		}, true
	} else if len(parts) >= 2 {
		return UpdatablePackage{
			Name:       parts[0],
			OldVersion: parts[1],
			NewVersion: "",
			Manager:    mgrName,
		}, true
	}
	return UpdatablePackage{}, false
}

func getExplicitPackageNames(foreignOnly bool) map[string]bool {
	args := []string{"-Qeq"}
	if foreignOnly {
		args = []string{"-Qemq"}
	}
	out, err := exec.Command("pacman", args...).Output()
	if err != nil {
		return nil
	}
	m := make(map[string]bool)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			m[name] = true
		}
	}
	return m
}

func (p *Pacman) GetUpdatable() ([]UpdatablePackage, error) {
	var out []byte
	var err error

	if _, lookErr := exec.LookPath("checkupdates"); lookErr == nil {
		cmd := exec.Command("checkupdates")
		out, err = cmd.Output()
		if err != nil && len(out) == 0 {
			cmd2 := exec.Command("pacman", "-Qu")
			out, _ = cmd2.Output()
		}
	} else {
		cmd := exec.Command("pacman", "-Qu")
		out, _ = cmd.Output()
	}

	explicitNames := getExplicitPackageNames(false)

	var results []UpdatablePackage
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		if u, ok := parseCheckupdatesLine(scanner.Text(), "pacman"); ok {
			if explicitNames == nil || explicitNames[u.Name] {
				results = append(results, u)
			}
		}
	}
	return results, nil
}

func (p *Pacman) Search(query string) ([]Package, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, nil
	}

	words := strings.Fields(trimmed)
	var args []string
	if len(words) > 1 {
		args = append([]string{"-Ss", "--color=never"}, words...)
	} else {
		args = []string{"-Ss", "--color=never", trimmed}
	}

	out, err := runCmdAllowExit1("pacman", args...)
	if err != nil {
		return nil, err
	}

	pkgs := parsePacmanSs(out)

	// If multi-word search returned nothing, try first token
	if len(pkgs) == 0 && len(words) > 1 {
		if out, err = runCmdAllowExit1("pacman", "-Ss", "--color=never", words[0]); err == nil {
			pkgs = parsePacmanSs(out)
		}
	}

	// If query yielded 0 results, build a regex fallback matching characters sequentially
	if len(pkgs) == 0 && len(trimmed) >= 3 && !strings.ContainsAny(trimmed, ".*+?^$[](){}|\\") {
		var regexPattern strings.Builder
		for i, r := range trimmed {
			if i > 0 {
				regexPattern.WriteString(".*")
			}
			regexPattern.WriteRune(r)
		}
		if out, err = runCmdAllowExit1("pacman", "-Ss", "--color=never", regexPattern.String()); err == nil {
			pkgs = parsePacmanSs(out)
		}
	}

	// Apply fuzzy ranking so best exact and fuzzy matches appear at the top
	return RankSearchResults(trimmed, pkgs), nil
}

// parsePacmanSs parses `pacman -Ss` or `yay -Ss` / `paru -Ss` output.
// Format alternates between:
//
//	repo/name version [flags]
//	    Description text
func parsePacmanSs(data []byte) []Package {
	lines := strings.Split(string(data), "\n")
	var pkgs []Package
	var cur *Package

	for _, line := range lines {
		if line == "" {
			continue
		}
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			// Description continuation
			if cur != nil {
				desc := strings.TrimSpace(line)
				cur.Description = desc
				pkgs = append(pkgs, *cur)
				cur = nil
			}
		} else {
			// Package line: "repo/name version [installed] ..."
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			namePart := parts[0]
			repo := ""
			if idx := strings.Index(namePart, "/"); idx >= 0 {
				repo = namePart[:idx]
				namePart = namePart[idx+1:]
			}
			isInstalled := strings.Contains(strings.ToLower(line), "installed")
			cur = &Package{
				Name:        namePart,
				Version:     parts[1],
				Repository:  repo,
				IsInstalled: isInstalled,
			}
		}
	}
	// Flush last package if it had no description line
	if cur != nil {
		pkgs = append(pkgs, *cur)
	}
	return pkgs
}

func (p *Pacman) UninstallOrphansCmd() []string {
	cmdStr := `orphans=$(pacman -Qtdq); if [ -n "$orphans" ]; then pacman -Rns --noconfirm $orphans; else echo "No orphan packages found."; fi`
	return []string{"sh", "-c", cmdStr}
}

func (p *Pacman) GetOrphans() ([]string, error) {
	out, err := runCmdAllowExit1("pacman", "-Qtdq")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	var orphans []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			orphans = append(orphans, l)
		}
	}
	return orphans, nil
}

func (p *Pacman) ListAll() ([]Package, error) {
	allOut, err := runCmd("pacman", "-Qin")
	if err != nil {
		return nil, err
	}

	pkgs, err := parsePacmanQi(allOut)
	if err != nil {
		return nil, err
	}

	// Identify system packages (base and base-devel dependencies)
	systemNames := make(map[string]bool)
	systemNames["base"] = true
	systemNames["base-devel"] = true

	for _, pkg := range pkgs {
		if pkg.Name == "base" || pkg.Name == "base-devel" {
			for _, d := range pkg.Dependencies {
				systemNames[d] = true
			}
		}
	}

	for i := range pkgs {
		if systemNames[pkgs[i].Name] {
			pkgs[i].IsSystem = true
		}
		if pkgs[i].Repository == "" {
			pkgs[i].Repository = "pacman"
		}
	}

	return pkgs, nil
}

func (p *Pacman) GetPackage(name string) (*Package, error) {
	out, err := runCmd("pacman", "-Qi", name)
	if err != nil {
		return nil, err
	}
	pkgs, err := parsePacmanQi(out)
	if err != nil || len(pkgs) == 0 {
		return nil, err
	}
	return &pkgs[0], nil
}

func parsePacmanQi(data []byte) ([]Package, error) {
	var pkgs []Package
	var cur *Package
	var lastKey string

	finalize := func() {
		if cur != nil {
			pkgs = append(pkgs, *cur)
			cur = nil
		}
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			finalize()
			lastKey = ""
			continue
		}

		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			if cur != nil && lastKey != "" {
				val := strings.TrimSpace(line)
				if val != "" && val != "None" {
					appendField(cur, lastKey, val)
				}
			}
			continue
		}

		before, after, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key := strings.TrimSpace(before)
		val := strings.TrimSpace(after)
		lastKey = key

		if key == "Name" {
			finalize()
			cur = &Package{Name: val}
			continue
		}
		if cur == nil {
			continue
		}

		switch key {
		case "Version":
			cur.Version = val
		case "Description":
			cur.Description = val
		case "Architecture":
			cur.Architecture = val
		case "Installed Size":
			cur.Size = parseSize(val)
		case "Install Date":
			cur.InstallDate = parseDate(val)
		case "Build Date":
			cur.BuildDate = parseDate(val)
		case "Install Reason":
			cur.InstallReason = val
		case "Depends On":
			if val != "None" {
				cur.Dependencies = append(cur.Dependencies, strings.Fields(val)...)
			}
		case "Optional Deps":
			if val != "None" {
				cur.OptDeps = append(cur.OptDeps, parseOptDepLine(val)...)
			}
		case "Optional For":
			if val != "None" {
				cur.OptFor = append(cur.OptFor, strings.Fields(val)...)
			}
		}
	}
	finalize()
	return pkgs, nil
}

func appendField(cur *Package, key, val string) {
	switch key {
	case "Depends On":
		cur.Dependencies = append(cur.Dependencies, strings.Fields(val)...)
	case "Optional Deps":
		cur.OptDeps = append(cur.OptDeps, parseOptDepLine(val)...)
	case "Optional For":
		cur.OptFor = append(cur.OptFor, strings.Fields(val)...)
	}
}

func parseOptDepLine(s string) []string {
	// "python: Python language support [installed]" → "python"
	var deps []string
	if i := strings.Index(s, ":"); i > 0 {
		deps = append(deps, strings.TrimSpace(s[:i]))
	} else if s != "" && s != "None" {
		deps = append(deps, strings.Fields(s)...)
	}
	return deps
}

func parseSize(s string) int64 {
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return 0
	}
	val, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	switch parts[1] {
	case "GiB":
		return int64(val * 1024 * 1024 * 1024)
	case "MiB":
		return int64(val * 1024 * 1024)
	case "KiB":
		return int64(val * 1024)
	default:
		return int64(val)
	}
}

func parseDate(s string) time.Time {
	formats := []string{
		"Mon 02 Jan 2006 03:04:05 PM MST",
		"Mon 02 Jan 2006 15:04:05 MST",
		"Mon Jan 02 15:04:05 2006",
	}
	for _, f := range formats {
		t, err := time.Parse(f, s)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

func runCmd(name string, args ...string) ([]byte, error) {
	var buf bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func runCmdAllowExit1(name string, args ...string) ([]byte, error) {
	var buf bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &buf
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return buf.Bytes(), nil
		}
		return nil, err
	}
	return buf.Bytes(), nil
}
