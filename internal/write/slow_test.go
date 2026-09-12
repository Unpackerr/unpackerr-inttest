package write_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Unpackerr/unpackerr-inttest/internal/write"
)

func TestFileChunked(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "out.bin")
	want := []byte("abcdefghijklmnop")
	start := time.Now()

	if err := write.File(path, want, 4, 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	if time.Since(start) < 40*time.Millisecond {
		t.Fatalf("expected pauses between chunks, elapsed %s", time.Since(start))
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(want) {
		t.Fatalf("got %q", got)
	}
}
