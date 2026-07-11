package incidents

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

type Incident struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Status      string   `json:"status"`
	Impact      string   `json:"impact"`
	Components  []string `json:"components"`
	StartedAt   string   `json:"started_at"`
	ResolvedAt  string   `json:"resolved_at,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Updates     []Update `json:"updates"`
	StartedTime time.Time
}

type Update struct {
	Status    string `json:"status"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func LoadDir(dataDir string) ([]Incident, error) {
	incidentDir := filepath.Join(dataDir, "incidents")
	matches, err := filepath.Glob(filepath.Join(incidentDir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)

	var all []Incident
	for _, path := range matches {
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		loaded, err := parseFile(body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		all = append(all, loaded...)
	}

	for i := range all {
		if err := all[i].normalize(); err != nil {
			return nil, err
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].StartedTime.After(all[j].StartedTime)
	})
	return all, nil
}

func Active(all []Incident) []Incident {
	var active []Incident
	for _, incident := range all {
		if incident.Status != "resolved" && incident.Status != "completed" {
			active = append(active, incident)
		}
	}
	return active
}

func parseFile(body []byte) ([]Incident, error) {
	var incident Incident
	if err := json.Unmarshal(body, &incident); err == nil && incident.ID != "" {
		return []Incident{incident}, nil
	}

	var incidents []Incident
	if err := json.Unmarshal(body, &incidents); err != nil {
		return nil, err
	}
	return incidents, nil
}

func (i *Incident) normalize() error {
	var messages []string
	if strings.TrimSpace(i.ID) == "" {
		messages = append(messages, "incident.id is required")
	}
	if strings.TrimSpace(i.Title) == "" {
		messages = append(messages, "incident.title is required")
	}
	if !slices.Contains([]string{"investigating", "identified", "monitoring", "resolved", "completed"}, i.Status) {
		messages = append(messages, "incident.status is invalid")
	}
	if !slices.Contains([]string{"minor", "degraded", "major", "critical", "maintenance"}, i.Impact) {
		messages = append(messages, "incident.impact is invalid")
	}
	if i.StartedAt == "" {
		messages = append(messages, "incident.started_at is required")
	} else {
		started, err := time.Parse(time.RFC3339, i.StartedAt)
		if err != nil {
			messages = append(messages, "incident.started_at must be RFC3339")
		}
		i.StartedTime = started
	}
	if i.ResolvedAt != "" {
		if _, err := time.Parse(time.RFC3339, i.ResolvedAt); err != nil {
			messages = append(messages, "incident.resolved_at must be RFC3339")
		}
	}
	for updateIndex, update := range i.Updates {
		if update.CreatedAt == "" {
			messages = append(messages, fmt.Sprintf("incident.updates[%d].created_at is required", updateIndex))
		} else if _, err := time.Parse(time.RFC3339, update.CreatedAt); err != nil {
			messages = append(messages, fmt.Sprintf("incident.updates[%d].created_at must be RFC3339", updateIndex))
		}
	}
	if len(messages) > 0 {
		return errors.New(strings.Join(messages, "; "))
	}
	return nil
}
