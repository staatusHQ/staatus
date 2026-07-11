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
	Uptime90d   float64         `json:"uptime90d"`
	Check       *PublicCheck    `json:"check,omitempty"`
	Links       []config.Link   `json:"links,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	History     []history.Point `json:"history,omitempty"`
	Timeline    []TimelineDay   `json:"timeline"`
}

type PublicCheck struct {
	Type           string `json:"type"`
	URL            string `json:"url"`
	Method         string `json:"method"`
	ExpectedStatus int    `json:"expectedStatus"`
}

type TimelineDay struct {
	Date        string  `json:"date"`
	Status      string  `json:"status"`
	StatusLabel string  `json:"statusLabel"`
	Uptime      float64 `json:"uptime"`
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

	components := publicComponents(options.Config.Components, loadedIncidents, loadedHistory, now)
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

func publicComponents(components []config.Component, allIncidents []incidents.Incident, series history.Series, now time.Time) []PublicComponent {
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
			Uptime90d:   uptime90d(component.ID, allIncidents, series, now),
			Links:       component.Links,
			Tags:        component.Tags,
			History:     series[component.ID],
			Timeline:    timelineFor(component.ID, allIncidents, series, now),
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

func timelineFor(componentID string, allIncidents []incidents.Incident, series history.Series, now time.Time) []TimelineDay {
	start := dayStart(now.UTC()).AddDate(0, 0, -89)
	pointsByDay := map[string]history.Point{}
	for _, point := range series[componentID] {
		day := dayString(point.CheckedAt)
		if day == "" {
			continue
		}
		pointsByDay[day] = point
	}

	days := make([]TimelineDay, 0, 90)
	for offset := 0; offset < 90; offset++ {
		day := start.AddDate(0, 0, offset)
		key := day.Format(time.DateOnly)
		status := "operational"
		uptime := 100.0

		if point, ok := pointsByDay[key]; ok {
			status = normalizeHistoryStatus(point.Status)
			if point.Uptime > 0 {
				uptime = point.Uptime
			}
		}
		if incidentStatus := incidentStatusForDay(componentID, day, allIncidents, now); incidentStatus != "" {
			status = incidentStatus
			uptime = uptimeForStatus(status)
		}

		days = append(days, TimelineDay{
			Date:        key,
			Status:      status,
			StatusLabel: labelForStatus(status),
			Uptime:      uptime,
		})
	}
	return days
}

func uptime90d(componentID string, allIncidents []incidents.Incident, series history.Series, now time.Time) float64 {
	timeline := timelineFor(componentID, allIncidents, series, now)
	if len(timeline) == 0 {
		return 100
	}
	total := 0.0
	for _, day := range timeline {
		total += day.Uptime
	}
	return round2(total / float64(len(timeline)))
}

func incidentStatusForDay(componentID string, day time.Time, allIncidents []incidents.Incident, now time.Time) string {
	dayStart := dayStart(day)
	dayEnd := dayStart.AddDate(0, 0, 1)
	status := ""
	for _, incident := range allIncidents {
		if !slices.Contains(incident.Components, componentID) {
			continue
		}
		started, err := time.Parse(time.RFC3339, incident.StartedAt)
		if err != nil {
			continue
		}
		resolved := now
		if incident.ResolvedAt != "" {
			parsed, err := time.Parse(time.RFC3339, incident.ResolvedAt)
			if err != nil {
				continue
			}
			resolved = parsed
		}
		if started.Before(dayEnd) && resolved.After(dayStart) {
			candidate := statusForImpact(incident.Impact)
			if statusRank(candidate) > statusRank(status) {
				status = candidate
			}
		}
	}
	return status
}

func overallStatus(components []PublicComponent) OverallStatus {
	status := "operational"
	for _, component := range components {
		if statusRank(component.Status) > statusRank(status) {
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

func statusRank(status string) int {
	switch status {
	case "maintenance":
		return 1
	case "degraded":
		return 2
	case "partial_outage":
		return 3
	case "major_outage":
		return 4
	default:
		return 0
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

func normalizeHistoryStatus(status string) string {
	switch status {
	case "up":
		return "operational"
	case "down":
		return "major_outage"
	case "operational", "degraded", "partial_outage", "major_outage", "maintenance":
		return status
	default:
		return "operational"
	}
}

func uptimeForStatus(status string) float64 {
	switch status {
	case "major_outage":
		return 0
	case "partial_outage":
		return 75
	case "degraded":
		return 98.5
	case "maintenance":
		return 99
	default:
		return 100
	}
}

func dayString(value string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.DateOnly)
}

func dayStart(value time.Time) time.Time {
	year, month, day := value.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func writeJSON(path string, doc any) error {
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}
