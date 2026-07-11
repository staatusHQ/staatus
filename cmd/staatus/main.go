package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/staatusHQ/staatus/internal/config"
	"github.com/staatusHQ/staatus/internal/render"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "version":
		fmt.Println(version)
		return nil
	case "validate":
		return runValidate(args[1:])
	case "render":
		return runRender(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\nRun `staatus help` for usage.", args[0])
	}
}

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "staatus.yml", "path to staatus.yml")
	jsonOutput := fs.Bool("json", false, "print validation result as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	result := validationResult{
		Config: *configPath,
		Valid:  err == nil,
	}
	if err != nil {
		result.Errors = validationErrors(err)
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if encodeErr := enc.Encode(result); encodeErr != nil {
			return encodeErr
		}
		if err != nil {
			return errors.New("configuration is invalid")
		}
		return nil
	}

	if err != nil {
		fmt.Printf("Staatus config is invalid: %s\n", *configPath)
		for _, message := range result.Errors {
			fmt.Printf("- %s\n", message)
		}
		return errors.New("configuration is invalid")
	}

	fmt.Printf("Staatus config is valid: %s\n", *configPath)
	fmt.Printf("- page: %s\n", cfg.Page.Name)
	fmt.Printf("- components: %d\n", len(cfg.Components))
	fmt.Printf("- checks: %d\n", cfg.CheckCount())
	return nil
}

func runRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "staatus.yml", "path to staatus.yml")
	outputDir := fs.String("out", "web/public/api", "directory for public API JSON")
	dataDir := fs.String("data", "data", "directory containing incidents and history")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	manifest, err := render.Render(render.Options{
		Config:    cfg,
		OutputDir: *outputDir,
		DataDir:   *dataDir,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Rendered Staatus API files to %s\n", *outputDir)
	for _, file := range manifest.Files {
		fmt.Printf("- %s\n", file)
	}
	return nil
}

type validationResult struct {
	Config string   `json:"config"`
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

func validationErrors(err error) []string {
	var validationErr *config.ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Messages
	}
	return []string{err.Error()}
}

func printUsage() {
	fmt.Println(`Staatus is a GitHub-native static status page toolkit.

Usage:
  staatus version
  staatus validate [--config staatus.yml] [--json]
  staatus render [--config staatus.yml] [--data data] [--out web/public/api]`)
}
