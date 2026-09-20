//go:build integration

package test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Unpackerr/unpackerr-inttest/internal/fixtures"
	"github.com/Unpackerr/unpackerr-inttest/internal/harness"
	"github.com/Unpackerr/unpackerr-inttest/internal/starrfake"
)

func TestStarrPollUpdatedAt(t *testing.T) {
	t.Parallel()

	fake := newFake(t, starrfake.AppSonarr)
	h := startStarr(t, fake, nil)
	waitStarrPoll(t, fake)

	deadline := time.Now().Add(harness.ExtractTimeout)
	for time.Now().Before(deadline) {
		stats, err := h.Stats()
		if err != nil {
			t.Fatal(err)
		}

		for _, queue := range stats.StarrQueues {
			if !queue.UpdatedAt.IsZero() && queue.Error == "" {
				return
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	stats, _ := h.Stats()
	t.Fatalf("starrQueues updatedAt missing: %+v", stats)
}

func TestStarrWaitingNotRestoredAfterDrop(t *testing.T) {
	t.Parallel()

	fake := newFake(t, starrfake.AppSonarr)
	out := filepath.Join(t.TempDir(), fixtures.SceneName("WAITRST"))
	if err := fixtures.MustPayload(out); err != nil {
		t.Fatal(err)
	}

	title := fixtures.SceneName("WAITRST")
	rec := fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusCompleted,
		OutputPath: out,
	})

	h := startStarr(t, fake, nil)
	h.WaitQueue(t, title, "waiting", harness.ExtractTimeout)

	if !fake.Drop(rec.ID) {
		t.Fatal("drop")
	}

	h.Restart(t)
	h.AssertNeverQueue(t, title, harness.SkipTimeout)
}
