package pm

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"
)

// AUR implements the Manager interface for Arch User Repository packages.
type AUR struct {
	helper string // "yay" or "paru"
}

func NewAUR(helper string) *AUR {
	if helper == "" {
		if _, err := exec.LookPath("yay"); err == nil {
			helper = "yay"
		} else if _, err := exec.LookPath("paru"); err == nil {
			helper = "paru"
		}
	}
	return &AUR{helper: helper}
}

func (a *AUR) Name() string { return "aur" }

func (a *AUR) RequiresSudo() bool { return true }

func (a *AUR) UninstallCmd(names []string) []string {
	return append([]string{"pacman", "-Rns", "--noconfirm"}, names...)
}

func (a *AUR) InstallCmd(name string) []string {
	h := a.helper
	if h == "" {
		h = "yay"
	}
	if h == "yay" {
		return []string{h, "-S", "--noconfirm", "--sudoloop", "--answerclean=None", "--answerdiff=None", "--answeredit=None", "--answerupgrade=None", name}
	}
	if h == "paru" {
		return []string{h, "-S", "--noconfirm", "--sudoloop", "--skipreview", name}
	}
	return []string{h, "-S", "--noconfirm", name}
}

func (a *AUR) UpdateCmd() []string {
	h := a.helper
	if h == "" {
		h = "yay"
	}
	if h == "yay" {
		return []string{h, "-Sua", "--noconfirm", "--sudoloop", "--answerclean=None", "--answerdiff=None", "--answeredit=None", "--answerupgrade=None"}
	}
	if h == "paru" {
		return []string{h, "-Sua", "--noconfirm", "--sudoloop", "--skipreview"}
	}
	return []string{h, "-Sua", "--noconfirm"}
}

func (a *AUR) UpdatePackagesCmd(names []string) []string {
	h := a.helper
	if h == "" {
		h = "yay"
	}
	if h == "yay" {
		return append([]string{h, "-S", "--noconfirm", "--sudoloop", "--answerclean=None", "--answerdiff=None", "--answeredit=None", "--answerupgrade=None"}, names...)
	}
	if h == "paru" {
		return append([]string{h, "-S", "--noconfirm", "--sudoloop", "--skipreview"}, names...)
	}
	return append([]string{h, "-S", "--noconfirm"}, names...)
}

func (a *AUR) GetUpdatable() ([]UpdatablePackage, error) {
	h := a.helper
	if h == "" {
		h = "yay"
	}

	var cmd *exec.Cmd
	if h == "yay" {
		cmd = exec.Command("yay", "-Qua")
	} else if h == "paru" {
		cmd = exec.Command("paru", "-Qua")
	} else {
		return nil, nil
	}

	out, _ := cmd.Output()
	var results []UpdatablePackage
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		if u, ok := parseCheckupdatesLine(scanner.Text(), "aur"); ok {
			results = append(results, u)
		}
	}
	return results, nil
}

func (a *AUR) UninstallOrphansCmd() []string {
	cmdStr := `orphans=$(pacman -Qtdqm); if [ -n "$orphans" ]; then pacman -Rns --noconfirm $orphans; else echo "No orphan packages found."; fi`
	return []string{"sh", "-c", cmdStr}
}

func (a *AUR) GetOrphans() ([]string, error) {
	out, err := runCmdAllowExit1("pacman", "-Qtdqm")
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

func (a *AUR) ListAll() ([]Package, error) {
	allOut, err := runCmd("pacman", "-Qim")
	if err != nil {
		return nil, err
	}

	pkgs, err := parsePacmanQi(allOut)
	if err != nil {
		return nil, err
	}

	for i := range pkgs {
		pkgs[i].Repository = "aur"
	}

	return pkgs, nil
}

func (a *AUR) GetPackage(name string) (*Package, error) {
	out, err := runCmd("pacman", "-Qim", name)
	if err != nil {
		return nil, err
	}
	pkgs, err := parsePacmanQi(out)
	if err != nil || len(pkgs) == 0 {
		return nil, err
	}
	pkgs[0].Repository = "aur"
	return &pkgs[0], nil
}

func (a *AUR) Search(query string) ([]Package, error) {
	h := a.helper
	if h == "" {
		h = "yay"
	}

	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, nil
	}

	words := strings.Fields(trimmed)
	var args []string
	if len(words) > 1 {
		args = append([]string{"-Ssa", "--color=never"}, words...)
	} else {
		args = []string{"-Ssa", "--color=never", trimmed}
	}

	out, err := runCmdAllowExit1(h, args...)
	if err != nil {
		return nil, err
	}

	pkgs := parsePacmanSs(out)

	// If multi-word search returned nothing, try first token
	if len(pkgs) == 0 && len(words) > 1 {
		if out, err = runCmdAllowExit1(h, "-Ssa", "--color=never", words[0]); err == nil {
			pkgs = parsePacmanSs(out)
		}
	}

	for i := range pkgs {
		if pkgs[i].Repository == "" {
			pkgs[i].Repository = "aur"
		}
	}

	return RankSearchResults(trimmed, pkgs), nil
}
