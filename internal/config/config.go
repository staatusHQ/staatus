package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type Config struct {
	Page       Page        `yaml:"page" json:"page"`
	Settings   Settings    `yaml:"settings" json:"settings"`
	Components []Component `yaml:"components" json:"components"`
}

type Page struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description,omitempty"`
	URL         string `yaml:"url" json:"url,omitempty"`
	Logo        string `yaml:"logo" json:"logo,omitempty"`
}

type Settings struct {
	Timezone         string `yaml:"timezone" json:"timezone,omitempty"`
	HistoryWindow    string `yaml:"history_window" json:"history_window,omitempty"`
	DefaultCheckTick string `yaml:"default_check_tick" json:"default_check_tick,omitempty"`
}

type Component struct {
	ID          string   `yaml:"id" json:"id"`
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description,omitempty"`
	Group       string   `yaml:"group" json:"group,omitempty"`
	Status      string   `yaml:"status" json:"status,omitempty"`
	Check       *Check   `yaml:"check" json:"check,omitempty"`
	Links       []Link   `yaml:"links" json:"links,omitempty"`
	Tags        []string `yaml:"tags" json:"tags,omitempty"`
}

type Check struct {
	Type           string            `yaml:"type" json:"type"`
	URL            string            `yaml:"url" json:"url"`
	Method         string            `yaml:"method" json:"method,omitempty"`
	Timeout        string            `yaml:"timeout" json:"timeout,omitempty"`
	ExpectedStatus int               `yaml:"expected_status" json:"expected_status,omitempty"`
	Headers        map[string]string `yaml:"headers" json:"headers,omitempty"`
}

type Link struct {
	Label string `yaml:"label" json:"label"`
	URL   string `yaml:"url" json:"url"`
}

type ValidationError struct {
	Messages []string
}

func (e *ValidationError) Error() string {
	return strings.Join(e.Messages, "; ")
}

func Load(path string) (*Config, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	decoder := yaml.NewDecoder(strings.NewReader(string(body)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) CheckCount() int {
	count := 0
	for _, component := range c.Components {
		if component.Check != nil {
			count++
		}
	}
	return count
}

func (c *Config) Validate() error {
	var messages []string

	if strings.TrimSpace(c.Page.Name) == "" {
		messages = append(messages, "page.name is required")
	}
	if c.Page.URL != "" && !validURL(c.Page.URL) {
		messages = append(messages, "page.url must be a valid URL")
	}
	if len(c.Components) == 0 {
		messages = append(messages, "at least one component is required")
	}

	seenIDs := map[string]bool{}
	for i, component := range c.Components {
		prefix := fmt.Sprintf("components[%d]", i)
		if !idPattern.MatchString(component.ID) {
			messages = append(messages, "%s.id must use lowercase letters, numbers, and hyphens", prefix)
		}
		if seenIDs[component.ID] {
			messages = append(messages, "%s.id %q is duplicated", prefix, component.ID)
		}
		seenIDs[component.ID] = true
		if strings.TrimSpace(component.Name) == "" {
			messages = append(messages, "%s.name is required", prefix)
		}
		if component.Status != "" && !slices.Contains(ComponentStatuses(), component.Status) {
			messages = append(messages, "%s.status must be one of %s", prefix, strings.Join(ComponentStatuses(), ", "))
		}
		if component.Check != nil {
			messages = append(messages, validateCheck(prefix+".check", *component.Check)...)
		}
		for j, link := range component.Links {
			linkPrefix := fmt.Sprintf("%s.links[%d]", prefix, j)
			if strings.TrimSpace(link.Label) == "" {
				messages = append(messages, linkPrefix+".label is required")
			}
			if !validURL(link.URL) {
				messages = append(messages, linkPrefix+".url must be a valid URL")
			}
		}
	}

	if len(messages) > 0 {
		return &ValidationError{Messages: messages}
	}
	return nil
}

func (c *Config) applyDefaults() {
	if c.Settings.Timezone == "" {
		c.Settings.Timezone = "UTC"
	}
	if c.Settings.HistoryWindow == "" {
		c.Settings.HistoryWindow = "90d"
	}
	if c.Settings.DefaultCheckTick == "" {
		c.Settings.DefaultCheckTick = "5m"
	}
	for i := range c.Components {
		if c.Components[i].Status == "" {
			c.Components[i].Status = "operational"
		}
		if c.Components[i].Check != nil {
			if c.Components[i].Check.Method == "" {
				c.Components[i].Check.Method = "GET"
			}
			if c.Components[i].Check.Timeout == "" {
				c.Components[i].Check.Timeout = "10s"
			}
			if c.Components[i].Check.ExpectedStatus == 0 {
				c.Components[i].Check.ExpectedStatus = 200
			}
		}
	}
}

func ComponentStatuses() []string {
	return []string{"operational", "degraded", "partial_outage", "major_outage", "maintenance"}
}

func validateCheck(prefix string, check Check) []string {
	var messages []string
	if check.Type != "http" {
		messages = append(messages, prefix+".type must be http")
	}
	if !validURL(check.URL) {
		messages = append(messages, prefix+".url must be a valid URL")
	}
	if check.Method != strings.ToUpper(check.Method) {
		messages = append(messages, prefix+".method must be uppercase")
	}
	if check.Timeout != "" {
		if _, err := time.ParseDuration(check.Timeout); err != nil {
			messages = append(messages, prefix+".timeout must be a Go duration like 10s")
		}
	}
	if check.ExpectedStatus < 100 || check.ExpectedStatus > 599 {
		messages = append(messages, prefix+".expected_status must be an HTTP status code")
	}
	return messages
}

func validURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}
