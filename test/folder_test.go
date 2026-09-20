//go:build integration

package test

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Unpackerr/unpackerr-inttest/internal/fixtures"
	"github.com/Unpackerr/unpackerr-inttest/internal/harness"
	"github.com/Unpackerr/unpackerr-inttest/internal/write"
)

func startFolder(t *testing.T, folder harness.Folder, tweak func(*harness.Options)) *harness.H {
	t.Helper()

	opts := harness.Options{
		Folders: []harness.Folder{folder},
	}
	if tweak != nil {
		tweak(&opts)
	}

	return harness.Start(t, opts)
}

func TestFolderSlowWriteStartDelay(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(root, "slow-src")
	if err := fixtures.Payload(src, 1, 16_384); err != nil {
		t.Fatal(err)
	}
	built := filepath.Join(root, "built.zip")
	fixtures.ZIP(t, built, src, "")

	data, err := os.ReadFile(built)
	if err != nil {
		t.Fatal(err)
	}

	h := startFolder(t, harness.Folder{Path: watch, DeleteAfter: 0}, nil)

	dest := filepath.Join(watch, "show.zip")
	done := make(chan error, 1)

	var writing atomic.Bool
	writing.Store(true)

	go func() {
		err := write.File(dest, data, 256, 400*time.Millisecond)
		writing.Store(false)
		done <- err
	}()

	for writing.Load() {
		assertNotExtracting(t, h, dest)
		time.Sleep(50 * time.Millisecond)
	}

	if err := <-done; err != nil {
		t.Fatal(err)
	}

	h.WaitQueue(t, dest, "extracted", harness.ExtractTimeout)
	assertExtractedMarker(t, watch)
}

func TestFolderDisableRecursion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	inner := fixtures.DirWithPayload(t, root, "inner")
	if err := os.WriteFile(filepath.Join(inner, "nested-only.txt"), []byte("inner payload\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	extra := t.TempDir()
	if err := os.WriteFile(filepath.Join(extra, "root.txt"), []byte("outer payload\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	staged := filepath.Join(root, "outer.zip")
	fixtures.NestedZIP(t, staged, inner, extra)

	h := startFolder(t, harness.Folder{
		Path:             watch,
		DisableRecursion: true,
		DeleteAfter:      0,
	}, nil)
	copyFile(t, staged, filepath.Join(watch, "outer.zip"))
	h.WaitQueue(t, filepath.Join(watch, "outer.zip"), "extracted", harness.ExtractTimeout)

	if fixtures.WalkHasBase(watch, "nested-only.txt") {
		t.Fatal("nested archive should not extract when disable_recursion=true")
	}

	if !fixtures.WalkHasBase(watch, "inner.zip") {
		t.Fatal("expected inner.zip to remain")
	}
}

func TestFolderRecursionExtractsNested(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	inner := fixtures.DirWithPayload(t, root, "inner")
	if err := os.WriteFile(filepath.Join(inner, "nested-only.txt"), []byte("inner payload\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	staged := filepath.Join(root, "outer.zip")
	fixtures.NestedZIP(t, staged, inner, "")

	h := startFolder(t, harness.Folder{Path: watch, DeleteAfter: 0}, nil)
	copyFile(t, staged, filepath.Join(watch, "outer.zip"))
	h.WaitQueue(t, filepath.Join(watch, "outer.zip"), "extracted", harness.ExtractTimeout)

	if !fixtures.WalkHasBase(watch, "nested-only.txt") {
		t.Fatal("expected nested-only.txt when recursion is enabled")
	}
}

func TestFolderMaxNested(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	wrap := filepath.Join(root, "wrap")
	if err := os.MkdirAll(wrap, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"one", "two", "three"} {
		src := fixtures.DirWithPayload(t, root, name+"-src")
		fixtures.ZIP(t, filepath.Join(wrap, name+".zip"), src, "")
	}

	staged := filepath.Join(root, "outer.zip")
	fixtures.ZIP(t, staged, wrap, "")

	h := startFolder(t, harness.Folder{
		Path:        watch,
		MaxNested:   2,
		DeleteAfter: 0,
	}, func(opts *harness.Options) {
		opts.RetryDelay = time.Hour
	})
	copyFile(t, staged, filepath.Join(watch, "outer.zip"))
	h.WaitQueue(t, filepath.Join(watch, "outer.zip"), "extractfailed", harness.ExtractTimeout)
}

func TestFolderExtrasMaxDepth(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	innerDeep := fixtures.DirWithPayload(t, root, "deep-src")
	if err := os.WriteFile(filepath.Join(innerDeep, "deep.txt"), []byte("buried\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	innerShallow := fixtures.DirWithPayload(t, root, "near-src")
	if err := os.WriteFile(filepath.Join(innerShallow, "near.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	deepZip := filepath.Join(tmp, "deep.zip")
	nearZip := filepath.Join(tmp, "near.zip")
	fixtures.ZIP(t, deepZip, innerDeep, "")
	fixtures.ZIP(t, nearZip, innerShallow, "")

	outerSrc := filepath.Join(tmp, "outer")
	if err := os.MkdirAll(filepath.Join(outerSrc, "a", "b", "c"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(outerSrc, "shallow"), 0o755); err != nil {
		t.Fatal(err)
	}

	copyFile(t, deepZip, filepath.Join(outerSrc, "a", "b", "c", "inner.zip"))
	copyFile(t, nearZip, filepath.Join(outerSrc, "shallow", "sibling.zip"))
	if err := os.WriteFile(filepath.Join(outerSrc, "keep.txt"), []byte("surface\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	staged := filepath.Join(root, "outer.zip")
	fixtures.ZIP(t, staged, outerSrc, "")

	h := startFolder(t, harness.Folder{
		Path:           watch,
		MaxNested:      64,
		ExtrasMaxDepth: 2,
		DeleteAfter:    0,
	}, nil)
	copyFile(t, staged, filepath.Join(watch, "outer.zip"))
	h.WaitQueue(t, filepath.Join(watch, "outer.zip"), "extracted", harness.ExtractTimeout)

	if fixtures.WalkHasBase(watch, "deep.txt") {
		t.Fatal("deep.txt should be skipped at extras_max_depth=2")
	}

	if !fixtures.WalkHasBase(watch, "near.txt") {
		t.Fatal("expected near.txt from shallow/sibling.zip at extras_max_depth=2")
	}
}

func TestFolderR00SkippedWhenRarExists(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	src := fixtures.DirWithPayload(t, root, "r00-src")
	staged := filepath.Join(root, "show.rar")
	fixtures.RAR(t, staged, src, fixtures.RAROptions{})

	h := startFolder(t, harness.Folder{Path: watch, DeleteAfter: 0}, nil)
	copyFile(t, staged, filepath.Join(watch, "show.rar"))
	copyFile(t, staged, filepath.Join(watch, "show.r00"))
	h.WaitQueue(t, filepath.Join(watch, "show.rar"), "extracted", harness.ExtractTimeout)
	h.WaitQueue(t, filepath.Join(watch, "show.r00"), "extractednothing", harness.ExtractTimeout)
	h.AssertStatusNot(t, filepath.Join(watch, "show.r00"), "extracted", 2*time.Second)

	if !strings.Contains(h.Logs(), "rar file exists") {
		t.Fatalf("expected r00 skip log, got:\n%s", h.Logs())
	}

	fixtures.MustExist(t, filepath.Join(watch, "show.r00"))
	assertExtractedMarker(t, watch)
}

func TestFolderExcludePaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	skip := filepath.Join(watch, "skip")
	okDir := filepath.Join(watch, "ok")

	src := fixtures.DirWithPayload(t, root, "ex-src")
	hidden := filepath.Join(root, "hidden.zip")
	okZip := filepath.Join(root, "show.zip")
	fixtures.ZIP(t, hidden, src, "")
	fixtures.ZIP(t, okZip, src, "")

	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	h := startFolder(t, harness.Folder{
		Path:         watch,
		ExcludePaths: []string{"skip"},
		DeleteAfter:  0,
	}, nil)

	if err := os.MkdirAll(skip, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(okDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, hidden, filepath.Join(skip, "hidden.zip"))
	copyFile(t, okZip, filepath.Join(okDir, "show.zip"))
	h.WaitQueue(t, okDir, "extracted", harness.ExtractTimeout)
	h.AssertNeverQueue(t, skip, harness.SkipTimeout)
	assertExtractedMarker(t, watch)
}

func TestFolderExtractISOsFalse(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	item := filepath.Join(watch, "release")

	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	src := fixtures.DirWithPayload(t, root, "iso-src")
	staged := filepath.Join(root, "show.zip")
	fixtures.ZIP(t, staged, src, "")

	h := startFolder(t, harness.Folder{
		Path:        watch,
		ExtractISOs: false,
		DeleteAfter: 0,
	}, nil)

	if err := os.MkdirAll(item, 0o755); err != nil {
		t.Fatal(err)
	}

	copyFile(t, staged, filepath.Join(item, "show.zip"))
	fixtures.DummyISO(t, filepath.Join(item, "disc.iso"))
	h.WaitQueue(t, item, "extracted", harness.ExtractTimeout)
	assertExtractedMarker(t, watch)
	fixtures.MustExist(t, filepath.Join(item, "disc.iso"))
}

func copyFile(t *testing.T, src, dest string) {
	t.Helper()

	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(dest, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertNotExtracting(t *testing.T, h *harness.H, want string) {
	t.Helper()

	item, ok, err := h.FindQueue(want)
	if err != nil {
		t.Fatal(err)
	}

	if !ok || item.Status == "waiting" {
		return
	}

	t.Fatalf("extract started while writing: %+v", item)
}
