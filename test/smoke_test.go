//go:build integration

package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Unpackerr/unpackerr-inttest/internal/fixtures"
	"github.com/Unpackerr/unpackerr-inttest/internal/harness"
)

func TestHarnessServesAPI(t *testing.T) {
	t.Parallel()

	h := harness.Start(t, harness.Options{})
	if _, err := h.Queue(); err != nil {
		t.Fatal(err)
	}
}

func TestPayloadMarkerWalk(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := fixtures.MustPayload(dir); err != nil {
		t.Fatal(err)
	}

	got, ok, err := fixtures.WalkReadBase(dir, fixtures.MarkerName)
	if err != nil || !ok {
		t.Fatalf("marker: ok=%v err=%v", ok, err)
	}

	if got != fixtures.Marker {
		t.Fatalf("marker %q", got)
	}

	if _, err := os.Stat(filepath.Join(dir, "release.nfo")); err != nil {
		t.Fatal(err)
	}
}
