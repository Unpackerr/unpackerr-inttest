// Package inject stages download layouts and POSTs them into cmd/faker.
package inject

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Unpackerr/unpackerr-inttest/internal/starrfake"
)

const (
	defaultFaker = "http://127.0.0.1:8989"
	defaultDir   = "downloads"
	defaultSize  = "4k"
)

// Config is one inject invocation.
type Config struct {
	Apps     []string
	Title    string
	Path     string
	Builtin  string
	Dir      string
	Faker    string
	Status   string
	Protocol string
	Size     int
	Compress bool
	Stdout   io.Writer
	Stderr   io.Writer
}

func usage(flags *flag.FlagSet) {
	_, _ = fmt.Fprintf(flags.Output(), `Usage:
  inject -a APP -t TITLE -p PATH [options]
  inject -a APP -b PATTERN [options]

-a is sonarr, radarr, lidarr, readarr, a comma list, or all.
-p (existing download folder) and -t are mutually exclusive with -b.
-b stages a known layout under -d (default ./%s) and POSTs it into faker.

Patterns: %s

Unpackerr [[APP]] url must be {faker}/{app} (example %s/sonarr).

`, defaultDir, strings.Join(PatternNames(), ", "), defaultFaker)
	flags.PrintDefaults()
}

// ParseFlags reads CLI args. args should not include the program name.
func ParseFlags(args []string, stdout, stderr io.Writer) (*Config, error) {
	cfg := &Config{
		Faker:    defaultFaker,
		Dir:      defaultDir,
		Status:   starrfake.StatusCompleted,
		Protocol: starrfake.ProtocolTorrent,
		Stdout:   stdout,
		Stderr:   stderr,
	}

	flags := flag.NewFlagSet("inject", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { usage(flags) }

	var (
		app    string
		size   string
		status string
		proto  string
	)

	flags.StringVar(&app, "a", "", "starr app: sonarr, radarr, lidarr, readarr, comma list, or all")
	flags.StringVar(&cfg.Title, "t", "", "queue title (required with -p; forbidden with -b)")
	flags.StringVar(&cfg.Path, "p", "", "existing folder to use as Starr outputPath")
	flags.StringVar(&cfg.Builtin, "b", "", "pre-built test pattern (mutually exclusive with -p/-t)")
	flags.StringVar(&cfg.Dir, "d", defaultDir, "directory to write -b layouts into")
	flags.StringVar(&cfg.Faker, "u", defaultFaker, "faker base URL")
	flags.StringVar(&status, "status", starrfake.StatusCompleted, "queue status: completed or downloading")
	flags.StringVar(&proto, "protocol", starrfake.ProtocolTorrent, "torrent or usenet")
	flags.StringVar(&size, "size", defaultSize, "payload size for -b (4k, 32m, 1g, or bytes)")
	flags.BoolVar(&cfg.Compress, "compress", false,
		"use real compression on -b archives (slower extract, visible progress)")

	if err := flags.Parse(args); err != nil {
		return nil, err
	}

	if strings.TrimSpace(app) == "" {
		return nil, errors.New("need -a (sonarr, radarr, lidarr, readarr, or all)")
	}

	apps, err := ParseApps(app)
	if err != nil {
		return nil, err
	}

	cfg.Apps = apps
	cfg.Status = strings.ToLower(strings.TrimSpace(status))
	cfg.Protocol = strings.ToLower(strings.TrimSpace(proto))

	if cfg.Status != starrfake.StatusCompleted && cfg.Status != starrfake.StatusDownloading {
		return nil, fmt.Errorf("invalid -status %q (completed or downloading)", status)
	}

	if cfg.Protocol != starrfake.ProtocolTorrent && cfg.Protocol != starrfake.ProtocolUsenet {
		return nil, fmt.Errorf("invalid -protocol %q (torrent or usenet)", proto)
	}

	if cfg.Builtin != "" && (cfg.Path != "" || cfg.Title != "") {
		return nil, errors.New("-b cannot be used with -p or -t")
	}

	if cfg.Builtin == "" && cfg.Path == "" {
		return nil, errors.New("need -b PATTERN or -t TITLE -p PATH")
	}

	if cfg.Path != "" && cfg.Title == "" {
		return nil, errors.New("-p requires -t")
	}

	if cfg.Builtin != "" {
		if _, ok := patterns[strings.ToLower(cfg.Builtin)]; !ok {
			return nil, fmt.Errorf("unknown -b %q (want %s)", cfg.Builtin, strings.Join(PatternNames(), ", "))
		}

		cfg.Size, err = ParseSize(size)
		if err != nil {
			return nil, err
		}
	}

	cfg.Faker = strings.TrimRight(cfg.Faker, "/")
	cfg.Builtin = strings.ToLower(cfg.Builtin)

	return cfg, nil
}

// ParseApps expands -a into one or more valid app names.
func ParseApps(raw string) ([]string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "all" {
		return starrfake.Apps(), nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}

	for _, part := range parts {
		app := strings.TrimSpace(part)
		if app == "" {
			continue
		}

		if app == "all" {
			return starrfake.Apps(), nil
		}

		if !starrfake.ValidApp(app) {
			return nil, fmt.Errorf("unknown app %q", app)
		}

		if _, ok := seen[app]; ok {
			continue
		}

		seen[app] = struct{}{}
		out = append(out, app)
	}

	if len(out) == 0 {
		return nil, errors.New("need at least one app in -a")
	}

	return out, nil
}
