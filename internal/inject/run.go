package inject

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Unpackerr/unpackerr-inttest/internal/starrfake"
)

// Main is cmd/inject.
func Main(args []string, stdout, stderr io.Writer) error {
	cfg, err := ParseFlags(args, stdout, stderr)
	if err != nil {
		return err
	}

	return Run(cfg)
}

// Run stages files if needed and POSTs one queue row per app.
func Run(cfg *Config) error {
	for _, app := range cfg.Apps {
		title := cfg.Title
		outputPath := cfg.Path

		if cfg.Builtin != "" {
			var err error

			title, outputPath, err = Stage(cfg.Builtin, cfg.Dir, app, cfg.Size, cfg.Compress)
			if err != nil {
				return fmt.Errorf("%s: %w", app, err)
			}
		} else {
			abs, err := filepath.Abs(outputPath)
			if err != nil {
				return err
			}

			stat, err := os.Stat(abs)
			if err != nil {
				return fmt.Errorf("-p %s: %w", outputPath, err)
			}

			if !stat.IsDir() {
				return fmt.Errorf("-p %s is not a directory", outputPath)
			}

			outputPath = abs
		}

		rec, err := Add(cfg.Faker, app, decorate(app, starrfake.Record{
			Title:      title,
			Status:     cfg.Status,
			Protocol:   cfg.Protocol,
			OutputPath: outputPath,
		}))
		if err != nil {
			return fmt.Errorf("%s: %w", app, err)
		}

		enc := json.NewEncoder(cfg.Stdout)
		enc.SetIndent("", "  ")

		if err := enc.Encode(rec); err != nil {
			return err
		}
	}

	return nil
}
