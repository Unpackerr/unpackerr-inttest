//go:build integration

package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Unpackerr/unpackerr-inttest/internal/fixtures"
	"github.com/Unpackerr/unpackerr-inttest/internal/harness"
)

func TestFolderPollerUsesInterval(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	src := fixtures.DirWithPayload(t, root, "poll-src")
	staged := filepath.Join(root, "show.zip")
	fixtures.ZIP(t, staged, src, "")

	h := startFolder(t, harness.Folder{
		Key:         "watch",
		Path:        watch,
		Interval:    200 * time.Millisecond,
		DeleteAfter: 0,
	}, nil)

	if !strings.Contains(h.Logs(), "[Folder] Polling:") {
		t.Fatalf("expected poller log, got:\n%s", h.Logs())
	}

	copyFile(t, staged, filepath.Join(watch, "show.zip"))
	item := h.WaitQueue(t, filepath.Join(watch, "show.zip"), "extracted", harness.ExtractTimeout)
	if item.Event != "polling" {
		t.Fatalf("event %q want polling: %+v", item.Event, item)
	}

	assertExtractedMarker(t, watch)
}

func TestFolderNestedArchiveAtStartIsIgnored(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	nested := filepath.Join(watch, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	src := fixtures.DirWithPayload(t, root, "nest-src")
	fixtures.ZIP(t, filepath.Join(nested, "show.zip"), src, "")

	h := startFolder(t, harness.Folder{
		Path:        watch,
		Interval:    200 * time.Millisecond,
		DeleteAfter: 0,
	}, nil)
	h.AssertNeverQueue(t, nested, harness.SkipTimeout)
}

func TestFolderWaitExtensions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	item := filepath.Join(watch, "Pending.Download")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	src := fixtures.DirWithPayload(t, root, "wait-src")
	staged := filepath.Join(root, "show.zip")
	fixtures.ZIP(t, staged, src, "")

	h := startFolder(t, harness.Folder{
		Path:           watch,
		WaitExtensions: []string{".part"},
		DeleteAfter:    0,
	}, nil)

	if err := os.MkdirAll(item, 0o755); err != nil {
		t.Fatal(err)
	}

	copyFile(t, staged, filepath.Join(item, "show.zip"))
	if err := os.WriteFile(filepath.Join(item, "show.zip.part"), []byte("downloading"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := h.WaitQueueFunc(t, item, harness.ExtractTimeout, func(row harness.QueueItem) bool {
		return row.Status == "waiting" && strings.Contains(row.Note, ".part") && row.DueKind == "start"
	})
	if got.Note != "show.zip.part" {
		t.Fatalf("note %+v", got)
	}

	h.AssertStatusNot(t, item, "extracted", 3*time.Second)

	if err := os.Remove(filepath.Join(item, "show.zip.part")); err != nil {
		t.Fatal(err)
	}

	h.WaitQueue(t, item, "extracted", harness.ExtractTimeout)
	assertExtractedMarker(t, watch)
}

func TestFolderWaitExtensionsIgnoresNestedPart(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	item := filepath.Join(watch, "Release")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	src := fixtures.DirWithPayload(t, root, "nested-part-src")
	staged := filepath.Join(root, "show.zip")
	fixtures.ZIP(t, staged, src, "")

	h := startFolder(t, harness.Folder{
		Path:           watch,
		WaitExtensions: []string{".part"},
		DeleteAfter:    0,
	}, nil)

	if err := os.MkdirAll(filepath.Join(item, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}

	copyFile(t, staged, filepath.Join(item, "show.zip"))
	if err := os.WriteFile(filepath.Join(item, "nested", "show.zip.part"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	h.WaitQueue(t, item, "extracted", harness.ExtractTimeout)
	assertExtractedMarker(t, watch)
}

func TestFolderSkipEmpty(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	item := filepath.Join(watch, "NoArchives")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	h := startFolder(t, harness.Folder{
		Path:        watch,
		SkipEmpty:   true,
		DeleteAfter: 0,
	}, nil)

	if err := os.MkdirAll(item, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(item, "movie.mkv"), []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}

	h.WaitQueue(t, item, "waiting", harness.ExtractTimeout)

	deadline := time.Now().Add(harness.ExtractTimeout)
	for time.Now().Before(deadline) {
		hist, err := h.History()
		if err != nil {
			t.Fatal(err)
		}

		for _, rec := range hist {
			if strings.Contains(rec.ID, item) || strings.Contains(rec.Path, item) {
				t.Fatalf("skip_empty wrote history: %+v", rec)
			}
		}

		row, ok, err := h.FindQueue(item)
		if err != nil {
			t.Fatal(err)
		}

		if ok && row.Status == "extracted" {
			t.Fatalf("skip_empty extracted: %+v", row)
		}

		if !ok && strings.Contains(h.Logs(), "Skipping empty folder") {
			return
		}

		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("folder was not skipped:\n%s", h.Logs())
}

func TestFolderExtractedDueKindCleanup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	src := fixtures.DirWithPayload(t, root, "due-src")
	staged := filepath.Join(root, "show.zip")
	fixtures.ZIP(t, staged, src, "")

	h := startFolder(t, harness.Folder{
		Path:        watch,
		DeleteAfter: 30 * time.Second,
		DeleteFiles: true,
	}, nil)
	copyFile(t, staged, filepath.Join(watch, "show.zip"))

	got := h.WaitQueue(t, filepath.Join(watch, "show.zip"), "extracted", harness.ExtractTimeout)
	if got.DueKind != "cleanup" || got.Due.IsZero() {
		t.Fatalf("due %+v", got)
	}
}

func TestFolderRestoreAfterRestart(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	src := fixtures.DirWithPayload(t, root, "restore-src")
	staged := filepath.Join(root, "show.zip")
	fixtures.ZIP(t, staged, src, "")
	dest := filepath.Join(watch, "show.zip")

	folder := harness.Folder{
		Path:        watch,
		DeleteAfter: 20 * time.Second,
		DeleteFiles: true,
	}
	h := startFolder(t, folder, nil)
	copyFile(t, staged, dest)
	h.WaitQueue(t, dest, "extracted", harness.ExtractTimeout)

	h.Restart(t)

	got := h.WaitQueue(t, dest, "extracted", harness.ExtractTimeout)
	if got.DueKind != "cleanup" {
		t.Fatalf("restored due %+v", got)
	}

	h.WaitHistory(t, dest, "deleted", harness.ExtractTimeout)
}

func TestFolderRemovedWatchPathNotRestored(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watch := filepath.Join(root, "watch")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	src := fixtures.DirWithPayload(t, root, "drop-src")
	staged := filepath.Join(root, "show.zip")
	fixtures.ZIP(t, staged, src, "")
	dest := filepath.Join(watch, "show.zip")

	folder := harness.Folder{
		Path:        watch,
		DeleteAfter: time.Hour,
		DeleteFiles: true,
	}
	h := startFolder(t, folder, nil)
	copyFile(t, staged, dest)
	h.WaitQueue(t, dest, "extracted", harness.ExtractTimeout)

	h.WriteConfig(t, harness.Options{})
	h.Restart(t)
	h.AssertNeverQueue(t, dest, harness.SkipTimeout)
}

func TestFolderRetryPersistsWaiting(t *testing.T) {
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
	dest := filepath.Join(watch, "outer.zip")

	h := startFolder(t, harness.Folder{
		Path:        watch,
		MaxNested:   2,
		DeleteAfter: 0,
	}, func(opts *harness.Options) {
		opts.RetryDelay = time.Hour
		opts.StartDelay = 5 * time.Second
	})
	copyFile(t, staged, dest)

	failed := h.WaitQueue(t, dest, "extractfailed", harness.ExtractTimeout)
	h.Retry(t, failed.ID)
	h.WaitQueue(t, dest, "waiting", harness.ExtractTimeout)

	h.Restart(t)

	got := h.WaitQueue(t, dest, "waiting", 15*time.Second)
	if got.Status == "extractfailed" {
		t.Fatalf("restart restored failed row: %+v", got)
	}
}
