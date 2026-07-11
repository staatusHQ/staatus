package checks

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/staatusHQ/staatus/internal/config"
)

type Result struct {
	ComponentID string        `json:"component_id"`
	Status      string        `json:"status"`
	Latency     time.Duration `json:"latency"`
	CheckedAt   time.Time     `json:"checked_at"`
	Error       string        `json:"error,omitempty"`
}

func RunHTTP(ctx context.Context, component config.Component, client *http.Client) Result {
	result := Result{
		ComponentID: component.ID,
		Status:      "unknown",
		CheckedAt:   time.Now().UTC(),
	}
	if component.Check == nil {
		result.Error = "component has no check"
		return result
	}

	timeout, err := time.ParseDuration(component.Check.Timeout)
	if err != nil {
		result.Error = fmt.Sprintf("invalid timeout: %s", component.Check.Timeout)
		return result
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	method := component.Check.Method
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, component.Check.URL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	for name, value := range component.Check.Headers {
		req.Header.Set(name, value)
	}

	start := time.Now()
	resp, err := client.Do(req)
	result.Latency = time.Since(start)
	if err != nil {
		result.Status = "down"
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	expected := component.Check.ExpectedStatus
	if expected == 0 {
		expected = http.StatusOK
	}
	if resp.StatusCode == expected {
		result.Status = "up"
		return result
	}

	result.Status = "down"
	result.Error = fmt.Sprintf("expected HTTP %d, got %d", expected, resp.StatusCode)
	return result
}
