//go:build integration

package test

import (
	"path/filepath"
	"testing"

	"github.com/Unpackerr/unpackerr-inttest/internal/fixtures"
	"github.com/Unpackerr/unpackerr-inttest/internal/harness"
	"github.com/Unpackerr/unpackerr-inttest/internal/httpcap"
	"github.com/Unpackerr/unpackerr-inttest/internal/starrfake"
)

func TestStarrHookTitlesAndExtractData(t *testing.T) {
	t.Parallel()

	capture := httpcap.Start(t)
	fake := newFake(t, starrfake.AppSonarr)
	out, src := fixtures.DownloadSet(t, t.TempDir(), "HOOK")
	fixtures.ZIP(t, filepath.Join(out, "show.zip"), src, "")

	title := fixtures.SceneName("HOOK")
	rec := fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusCompleted,
		OutputPath: out,
	})

	h := startStarr(t, fake, func(opts *harness.Options) {
		opts.Webhooks = []harness.Webhook{{
			Key:    "capture",
			URL:    capture.URL,
			Events: []int{0},
		}}
		opts.HookIDs = map[string]string{"url": "https://unpackerr.example"}
		opts.HookTitles = map[string]string{"extracted": "Custom extracted"}
	})

	extracted := capture.WaitEvent(t, "extracted", harness.ExtractTimeout)
	if extracted["eventTitle"] != "Custom extracted" {
		t.Fatalf("extracted title %+v", extracted)
	}

	ids, _ := extracted["customIDs"].(map[string]any)
	if ids["url"] != "https://unpackerr.example" {
		t.Fatalf("customIDs %+v", extracted)
	}

	data, _ := extracted["data"].(map[string]any)
	if data == nil {
		t.Fatalf("extracted missing data %+v", extracted)
	}

	h.WaitQueue(t, title, "extracted", harness.ExtractTimeout)

	if !fake.Drop(rec.ID) {
		t.Fatal("drop")
	}

	imported := capture.WaitEvent(t, "imported", harness.ExtractTimeout)
	importedData, _ := imported["data"].(map[string]any)
	if importedData == nil {
		t.Fatalf("imported missing data %+v", imported)
	}
}
