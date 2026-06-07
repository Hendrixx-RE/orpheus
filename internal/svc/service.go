package svc

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"
)

type Service struct {
	Unit        string
	LoadState   string
	ActiveState string
	SubState    string
	Description string
	OwnerPkg    string
}

func (s *Service) IsActive() bool   { return s.ActiveState == "active" }
func (s *Service) IsFailed() bool   { return s.ActiveState == "failed" }
func (s *Service) StatusBadge() string {
	switch s.ActiveState {
	case "active":
		return "●"
	case "failed":
		return "✗"
	case "inactive":
		return "○"
	default:
		return "?"
	}
}

func ListServices() ([]Service, error) {
	var buf bytes.Buffer
	cmd := exec.Command("systemctl", "list-units", "--type=service", "--all",
		"--plain", "--no-legend", "--no-pager")
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var svcs []Service
	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		unit := fields[0]
		if !strings.HasSuffix(unit, ".service") {
			continue
		}
		desc := ""
		if len(fields) > 4 {
			desc = strings.Join(fields[4:], " ")
		}
		svcs = append(svcs, Service{
			Unit:        unit,
			LoadState:   fields[1],
			ActiveState: fields[2],
			SubState:    fields[3],
			Description: desc,
		})
	}
	return svcs, nil
}

func FindOwner(serviceName string) string {
	unitFile := "/usr/lib/systemd/system/" + serviceName
	var buf bytes.Buffer
	cmd := exec.Command("pacman", "-Qo", unitFile)
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return ""
	}
	// output: "/usr/lib/systemd/system/foo.service is owned by bar 1.2.3"
	parts := strings.Fields(strings.TrimSpace(buf.String()))
	if len(parts) >= 5 {
		return parts[4]
	}
	return ""
}

func EnrichWithOwners(svcs []Service) []Service {
	for i := range svcs {
		svcs[i].OwnerPkg = FindOwner(svcs[i].Unit)
	}
	return svcs
}
