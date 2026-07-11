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

func (p *Pacman) UninstallCmd(names []string) []string {
	args := []string{"pacman", "-Rns", "--noconfirm"}
	return append(args, names...)
}

func (p *Pacman) ListAll() ([]Package, error) {
	allOut, err := runCmd("pacman", "-Qi")
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

func GetOrphansDetailed() ([]Package, error) {
	out, _ := runCmdAllowExit1("pacman", "-Qdti")
	if len(out) == 0 {
		return nil, nil
	}
	return parsePacmanQi(out)
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
