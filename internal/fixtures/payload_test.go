package fixtures_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Unpackerr/unpackerr-inttest/internal/fixtures"
)

func TestPayloadSeeded(t *testing.T) {
	t.Parallel()

	a := t.TempDir()
	b := t.TempDir()

	if err := fixtures.Payload(a, 7, 64); err != nil {
		t.Fatal(err)
	}

	if err := fixtures.Payload(b, 7, 64); err != nil {
		t.Fatal(err)
	}

	aa, err := os.ReadFile(filepath.Join(a, "random.bin"))
	if err != nil {
		t.Fatal(err)
	}

	bb, err := os.ReadFile(filepath.Join(b, "random.bin"))
	if err != nil {
		t.Fatal(err)
	}

	if string(aa) != string(bb) {
		t.Fatal("seeded payload should be deterministic")
	}

	got, err := os.ReadFile(filepath.Join(a, fixtures.MarkerName))
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != fixtures.Marker {
		t.Fatalf("marker %q", got)
	}
}

func TestWalkReadBaseMissingRoot(t *testing.T) {
	t.Parallel()

	_, ok, err := fixtures.WalkReadBase(filepath.Join(t.TempDir(), "nope"), fixtures.MarkerName)
	if err == nil || ok {
		t.Fatal("missing root should return an error")
	}
}
