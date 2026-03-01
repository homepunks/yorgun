package report

import (
	"fmt"
	"strings"

	"github.com/homepunks/yorgun/config"
	"github.com/homepunks/yorgun/docker"
)

func FormatStartupReport(statuses []docker.ContainerStatus, cfg *config.Config) string {
	var b strings.Builder

	byService := make(map[string]docker.ContainerStatus)
	for _, s := range statuses {
		byService[s.Service] = s
	}

	problems := 0
	var lines []string

	for _, svc := range cfg.Services {
		s, found := byService[svc.Name]
		if !found {
			problems++
			lines = append(lines, fmt.Sprintf("  %-20s  NOT FOUND    container missing", svc.Name))
			continue
		}

		ok, problem := evaluate(s)
		state := strings.ToUpper(s.State)

		if ok {
			lines = append(lines, fmt.Sprintf("  %-20s  %-10s  %s", s.Service, state, s.Status))
		} else {
			problems++
			marker := "⚠ "
			if svc.Critical {
				marker = "✖"
			}
			lines = append(lines, fmt.Sprintf("  %s %-17s  %-10s  %s", marker, s.Service, state, problem))
		}
	}

	if problems == 0 {
		fmt.Fprintf(&b, "✓ yorgun — all %d containers healthy\n\n", len(cfg.Services))
	} else {
		fmt.Fprintf(&b, "⚠ yorgun — %d problem(s) detected\n\n", problems)
	}

	for _, line := range lines {
		b.WriteString(line + "\n")
	}

	return b.String()
}

func FormatEvent(service, action, exitCode string, critical bool) string {
	icon := "⚠"
	severity := ""

	switch action {
	case "die":
		icon = "☠"
		if critical {
			severity = " [CRITICAL]"
		}
	case "oom":
		icon = "☒"
		severity = " [OOM]"
	case "stop":
		icon = "■"
	case "start":
		icon = "▶"
	case "health_status: unhealthy":
		icon = "✚"
	case "health_status: healthy":
		icon = "✖"
	}

	msg := fmt.Sprintf("%s %s — %s%s", icon, service, action, severity)

	if exitCode != "" && exitCode != "0" {
		msg += fmt.Sprintf(" (exit code: %s)", exitCode)
	}

	return msg
}

func evaluate(s docker.ContainerStatus) (bool, string) {
	switch s.State {
	case "running":
		if s.Health == "unhealthy" {
			return false, "running but healthcheck failing"
		}
		if s.RestartCount > 3 {
			return false, fmt.Sprintf("running but restarted %d times", s.RestartCount)
		}
		return true, ""

	case "exited":
		reason := fmt.Sprintf("exited with code %d", s.ExitCode)
		if s.OOMKilled {
			reason += " (OOM killed)"
		}
		if s.Error != "" {
			reason += ": " + s.Error
		}
		return false, reason

	case "restarting":
		return false, fmt.Sprintf("restarting (restart count: %d)", s.RestartCount)

	case "dead":
		return false, "dead"

	case "paused":
		return false, "paused"

	default:
		return false, fmt.Sprintf("unknown state: %s", s.State)
	}
}
