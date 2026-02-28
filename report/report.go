package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/homepunks/yorgun/config"
	"github.com/homepunks/yorgun/docker"
)

type Verdict struct {
	docker.ContainerStatus
	OK       bool
	Critical bool
	Problem  string
}

type Report struct {
	Timestamp  time.Time
	Verdicts   []Verdict
	Missing    []string
	AllHealthy bool
	Problems   int
}

func Build(statuses []docker.ContainerStatus, cfg *config.Config) *Report {
	r := &Report{
		Timestamp:  time.Now(),
		AllHealthy: true,
	}

	byService := make(map[string]docker.ContainerStatus)
	for _, s := range statuses {
		byService[s.Service] = s
	}

	for _, svc := range cfg.Services {
		s, found := byService[svc.Name]
		if !found {
			r.Missing = append(r.Missing, svc.Name)
			r.AllHealthy = false
			r.Problems++
			continue
		}

		v := Verdict{
			ContainerStatus: s,
			Critical:        svc.Critical,
		}

		v.OK, v.Problem = evaluate(s)

		if !v.OK {
			r.AllHealthy = false
			r.Problems++
		}

		r.Verdicts = append(r.Verdicts, v)
	}

	return r
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

func (r *Report) FormatText() string {
	var b strings.Builder

	if r.AllHealthy {
		total := len(r.Verdicts)
		fmt.Fprintf(&b, "☑ yorgun — all %d containers healthy\n\n", total)
	} else {
		fmt.Fprintf(&b, "☒  yorgun — %d problem(s) detected\n\n", r.Problems)
	}

	for _, name := range r.Missing {
		fmt.Fprintf(&b, "  %-20s  NOT FOUND    container missing\n", name)
	}

	for _, v := range r.Verdicts {
		state := strings.ToUpper(v.State)
		health := ""
		if v.Health != "none" {
			health = "(" + v.Health + ")"
		}

		if v.OK {
			fmt.Fprintf(&b, "  %-20s  %-10s  %s %s\n",
				v.Service, state, v.Status, health)
		} else {
			marker := "! "
			if v.Critical {
				marker = "✗"
			}
			fmt.Fprintf(&b, "  %s %-17s  %-10s  %s\n",
				marker, v.Service, state, v.Problem)
		}
	}

	return b.String()
}

func (r *Report) HasCriticalFailure() bool {
	for _, name := range r.Missing {
		for _, v := range r.Verdicts {
			if v.Service == name && v.Critical {
				return true
			}
		}
	}

	for _, v := range r.Verdicts {
		if !v.OK && v.Critical {
			return true
		}
	}

	return false
}
