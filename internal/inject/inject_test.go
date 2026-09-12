package inject_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Unpackerr/unpackerr-inttest/internal/fixtures"
	"github.com/Unpackerr/unpackerr-inttest/internal/inject"
	"github.com/Unpackerr/unpackerr-inttest/internal/starrfake"
)

func TestParseFlagsModes(t *testing.T) {
	t.Parallel()

	_, err := inject.ParseFlags([]string{"-a", "sonarr", "-b", "zip", "-t", "Nope"}, os.Stdout, os.Stderr)
	if err == nil || !strings.Contains(err.Error(), "-b cannot") {
		t.Fatalf("expected -b/-t exclusion, got %v", err)
	}

	_, err = inject.ParseFlags([]string{"-a", "sonarr", "-b", "zip", "-p", "/tmp"}, os.Stdout, os.Stderr)
	if err == nil || !strings.Contains(err.Error(), "-b cannot") {
		t.Fatalf("expected -b/-p exclusion, got %v", err)
	}

	_, err = inject.ParseFlags([]string{"-a", "sonarr", "-p", "/tmp"}, os.Stdout, os.Stderr)
	if err == nil || !strings.Contains(err.Error(), "-p requires -t") {
		t.Fatalf("expected -t with -p, got %v", err)
	}

	_, err = inject.ParseFlags([]string{"-a", "sonarr"}, os.Stdout, os.Stderr)
	if err == nil {
		t.Fatal("expected missing mode")
	}

	_, err = inject.ParseFlags([]string{"-a", "plex", "-b", "zip"}, os.Stdout, os.Stderr)
	if err == nil || !strings.Contains(err.Error(), "unknown app") {
		t.Fatalf("expected unknown app, got %v", err)
	}

	_, err = inject.ParseFlags([]string{"-a", "sonarr", "-b", "iso"}, os.Stdout, os.Stderr)
	if err == nil || !strings.Contains(err.Error(), "unknown -b") {
		t.Fatalf("expected unknown pattern, got %v", err)
	}

	cfg, err := inject.ParseFlags([]string{"-a", "all", "-b", "recursive", "-size", "32k"}, os.Stdout, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Apps) != 4 || cfg.Builtin != "recursive" || cfg.Size != 32*1024 {
		t.Fatalf("%+v", cfg)
	}

	cfg, err = inject.ParseFlags([]string{
		"-a", "lidarr,sonarr", "-t", "Album", "-p", "/dl/Album", "-status", "downloading",
	}, os.Stdout, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Apps) != 2 || cfg.Apps[0] != starrfake.AppLidarr || cfg.Title != "Album" {
		t.Fatalf("%+v", cfg)
	}
}

func TestParseSize(t *testing.T) {
	t.Parallel()

	got, err := inject.ParseSize("4k")
	if err != nil || got != 4096 {
		t.Fatalf("4k: %d %v", got, err)
	}

	got, err = inject.ParseSize("2m")
	if err != nil || got != 2*1024*1024 {
		t.Fatalf("2m: %d %v", got, err)
	}

	if _, err := inject.ParseSize("0"); err == nil {
		t.Fatal("expected invalid 0")
	}

	if _, err := inject.ParseSize("3g"); err == nil {
		t.Fatal("expected 3g over cap")
	}

	got, err = inject.ParseSize("2g")
	if err != nil || got != 2<<30 {
		t.Fatalf("2g at cap: %d %v", got, err)
	}

	if _, err := inject.ParseSize("8589934592g"); err == nil {
		t.Fatal("expected overflowing g suffix to fail the 2GiB cap")
	}
}

func TestParseApps(t *testing.T) {
	t.Parallel()

	got, err := inject.ParseApps("ALL")
	if err != nil || len(got) != 4 {
		t.Fatalf("%v %v", got, err)
	}

	got, err = inject.ParseApps(strings.Join([]string{"radarr", "radarr", "lidarr"}, ","))
	if err != nil || strings.Join(got, ",") != "radarr,lidarr" {
		t.Fatalf("%v %v", got, err)
	}
}

func TestRunBuiltinZipAgainstHub(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("zip"); err != nil {
		t.Skip("zip not in PATH")
	}

	hub := starrfake.NewHub("k")
	srv := httptest.NewServer(hub.Handler())
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	stdout := &bytes.Buffer{}
	cfg, err := inject.ParseFlags([]string{
		"-a", "sonarr,radarr",
		"-b", "zip",
		"-d", dir,
		"-u", srv.URL,
		"-size", "64",
	}, stdout, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}

	if err := inject.Run(cfg); err != nil {
		if strings.Contains(err.Error(), "zip not in PATH") {
			t.Skip(err)
		}

		t.Fatal(err)
	}

	var rec starrfake.Record
	dec := json.NewDecoder(stdout)
	if err := dec.Decode(&rec); err != nil {
		t.Fatal(err)
	}

	if rec.ID == 0 || rec.OutputPath == "" || rec.Title == "" {
		t.Fatalf("sonarr record %+v", rec)
	}

	if _, err := os.Stat(filepath.Join(rec.OutputPath, "show.zip")); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/radarr/api/v3/queue", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("X-Api-Key", "k")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	var queue starrfake.Queue
	if err := json.NewDecoder(res.Body).Decode(&queue); err != nil {
		t.Fatal(err)
	}

	if queue.TotalRecords != 1 || queue.Records[0].MovieID != 1 {
		t.Fatalf("radarr queue %+v", queue)
	}
}

func TestRunExistingPath(t *testing.T) {
	t.Parallel()

	hub := starrfake.NewHub("k")
	srv := httptest.NewServer(hub.Handler())
	t.Cleanup(srv.Close)

	path := t.TempDir()
	if err := fixtures.MustPayload(path); err != nil {
		t.Fatal(err)
	}

	stdout := &bytes.Buffer{}
	cfg, err := inject.ParseFlags([]string{
		"-a", "readarr",
		"-t", "Book.Title-GROUP",
		"-p", path,
		"-u", srv.URL,
	}, stdout, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}

	if err := inject.Run(cfg); err != nil {
		t.Fatal(err)
	}

	snap := hub.App(starrfake.AppReadarr).Snapshot()
	if len(snap) != 1 || snap[0].Title != "Book.Title-GROUP" || snap[0].AuthorID != 1 {
		t.Fatalf("%+v", snap)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}

	if snap[0].OutputPath != abs {
		t.Fatalf("outputPath %q vs %q", snap[0].OutputPath, abs)
	}
}
