package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Point struct {
	ComponentID string  `json:"component_id"`
	Status      string  `json:"status"`
	CheckedAt   string  `json:"checked_at"`
	LatencyMS   int     `json:"latency_ms,omitempty"`
	Uptime      float64 `json:"uptime,omitempty"`
}

type Series map[string][]Point

func LoadDir(dataDir string) (Series, error) {
	matches, err := filepath.Glob(filepath.Join(dataDir, "history", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)

	series := Series{}
	for _, path := range matches {
		if err := loadFile(series, path); err != nil {
			return nil, err
		}
	}
	for componentID := range series {
		sort.SliceStable(series[componentID], func(i, j int) bool {
			return parseTime(series[componentID][i].CheckedAt).Before(parseTime(series[componentID][j].CheckedAt))
		})
	}
	return series, nil
}

func loadFile(series Series, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var point Point
		if err := json.Unmarshal(line, &point); err != nil {
			return err
		}
		if point.ComponentID == "" {
			continue
		}
		series[point.ComponentID] = append(series[point.ComponentID], point)
	}
	return scanner.Err()
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
