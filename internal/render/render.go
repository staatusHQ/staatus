package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/staatusHQ/staatus/internal/config"
	"github.com/staatusHQ/staatus/internal/history"
	"github.com/staatusHQ/staatus/internal/incidents"
)

const schemaVersion = "staatus.public.v1"

type Options struct {
	Config    *config.Config
	OutputDir string
	DataDir   string
	Now       time.Time
}

type Manifest struct {
	Files []string
}

type StatusDocument struct {
	SchemaVersion string         `json:"schemaVersion"`
	GeneratedAt   string         `json:"generatedAt"`
	Page          config.Page    `json:"page"`
	Overall       OverallStatus  `json:"overall"`
	Summary       Summary        `json:"summary"`
	LastUpdated   string         `json:"lastUpdated"`
	History       history.Series `json:"history,omitempty"`
}

type OverallStatus struct {
	Status string `json:"status"`
	Label  string `json:"label"`
}

type Summary struct {
	Components map[string]int `json:"components"`
	Incidents  map[string]int `json:"incidents"`
}

type ComponentsDocument struct {
	SchemaVersion string            `json:"schemaVersion"`
	GeneratedAt   string            `json:"generatedAt"`
	Components    []PublicComponent `json:"components"`
}

type PublicComponent struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Group       string          `json:"group,omitempty"`
	Status      string          `json:"status"`
	StatusLabel string          `json:"statusLabel"`
	Check       *PublicCheck    `json:"check,omitempty"`
	Links       []config.Link   `json:"links,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	History     []history.Point `json:"history,omitempty"`
}

type PublicCheck struct {
	Type           string `json:"type"`
	URL            string `json:"url"`
	Method         string `json:"method"`
	ExpectedStatus int    `json:"expectedStatus"`
}

type IncidentsDocument struct {
	SchemaVersion string               `json:"schemaVersion"`
	GeneratedAt   string               `json:"generatedAt"`
	Active        []incidents.Incident `json:"active"`
	Recent        []incidents.Incident `json:"recent"`
}

func Render(options Options) (*Manifest, error) {
	if options.Config == nil {
		return nil, fmt.Errorf("render config is required")
	}
	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if options.OutputDir == "" {
		options.OutputDir = "web/public/api"
	}
	if options.DataDir == "" {
		options.DataDir = "data"
	}

	loadedIncidents, err := incidents.LoadDir(options.DataDir)
	if err != nil {
		return nil, err
	}
	loadedHistory, err := history.LoadDir(options.DataDir)
	if err != nil {
		return nil, err
	}

	components := publicComponents(options.Config.Components, loadedIncidents, loadedHistory)
	statusDoc := StatusDocument{
		SchemaVersion: schemaVersion,
		GeneratedAt:   now.Format(time.RFC3339),
		Page:          options.Config.Page,
		Overall:       overallStatus(components),
		Summary: Summary{
			Components: componentCounts(components),
			Incidents: map[string]int{
				"active":   len(incidents.Active(loadedIncidents)),
				"resolved": countIncidents(loadedIncidents, "resolved"),
			},
		},
		LastUpdated: now.Format(time.RFC3339),
		History:     loadedHistory,
	}
	componentsDoc := ComponentsDocument{
		SchemaVersion: schemaVersion,
		GeneratedAt:   now.Format(time.RFC3339),
		Components:    components,
	}
	incidentsDoc := IncidentsDocument{
		SchemaVersion: schemaVersion,
		GeneratedAt:   now.Format(time.RFC3339),
		Active:        incidents.Active(loadedIncidents),
		Recent:        loadedIncidents,
	}

	files := []struct {
		name string
		doc  any
	}{
		{name: "status.json", doc: statusDoc},
		{name: "components.json", doc: componentsDoc},
		{name: "incidents.json", doc: incidentsDoc},
	}

	if err := os.MkdirAll(options.OutputDir, 0o755); err != nil {
		return nil, err
	}

	manifest := &Manifest{}
	for _, file := range files {
		path := filepath.Join(options.OutputDir, file.name)
		if err := writeJSON(path, file.doc); err != nil {
			return nil, err
		}
		manifest.Files = append(manifest.Files, path)
	}
	sort.Strings(manifest.Files)
	return manifest, nil
}

func publicComponents(components []config.Component, allIncidents []incidents.Incident, series history.Series) []PublicComponent {
	active := incidents.Active(allIncidents)
	public := make([]PublicComponent, 0, len(components))
	for _, component := range components {
		status := component.Status
		for _, incident := range active {
			if slices.Contains(incident.Components, component.ID) {
				status = statusForImpact(incident.Impact)
				break
			}
		}
		publicComponent := PublicComponent{
			ID:          component.ID,
			Name:        component.Name,
			Description: component.Description,
			Group:       component.Group,
			Status:      status,
			StatusLabel: labelForStatus(status),
			Links:       component.Links,
			Tags:        component.Tags,
			History:     series[component.ID],
		}
		if component.Check != nil {
			publicComponent.Check = &PublicCheck{
				Type:           component.Check.Type,
				URL:            component.Check.URL,
				Method:         component.Check.Method,
				ExpectedStatus: component.Check.ExpectedStatus,
			}
		}
		public = append(public, publicComponent)
	}
	return public
}

func overallStatus(components []PublicComponent) OverallStatus {
	rank := map[string]int{
		"operational":    0,
		"maintenance":    1,
		"degraded":       2,
		"partial_outage": 3,
		"major_outage":   4,
	}
	status := "operational"
	for _, component := range components {
		if rank[component.Status] > rank[status] {
			status = component.Status
		}
	}
	return OverallStatus{Status: status, Label: labelForStatus(status)}
}

func componentCounts(components []PublicComponent) map[string]int {
	counts := map[string]int{
		"operational":    0,
		"degraded":       0,
		"partial_outage": 0,
		"major_outage":   0,
		"maintenance":    0,
	}
	for _, component := range components {
		counts[component.Status]++
	}
	return counts
}

func countIncidents(all []incidents.Incident, status string) int {
	count := 0
	for _, incident := range all {
		if incident.Status == status {
			count++
		}
	}
	return count
}

func statusForImpact(impact string) string {
	switch impact {
	case "critical":
		return "major_outage"
	case "major":
		return "partial_outage"
	case "minor", "degraded":
		return "degraded"
	case "maintenance":
		return "maintenance"
	default:
		return "operational"
	}
}

func labelForStatus(status string) string {
	switch status {
	case "operational":
		return "Operational"
	case "degraded":
		return "Degraded performance"
	case "partial_outage":
		return "Partial outage"
	case "major_outage":
		return "Major outage"
	case "maintenance":
		return "Maintenance"
	default:
		return status
	}
}

func writeJSON(path string, doc any) error {
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}
