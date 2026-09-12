//go:build integration

package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Unpackerr/unpackerr-inttest/internal/fixtures"
	"github.com/Unpackerr/unpackerr-inttest/internal/harness"
	"github.com/Unpackerr/unpackerr-inttest/internal/starrfake"
)

func newFake(t *testing.T, app string) *starrfake.Server {
	t.Helper()

	fake := starrfake.New(app, harness.StarrAPIKey)
	fake.Start()
	t.Cleanup(fake.Close)

	return fake
}

func startStarr(t *testing.T, fake *starrfake.Server, tweak func(*harness.Options)) *harness.H {
	t.Helper()

	opts := harness.Options{
		Starr: []harness.Starr{{
			App:         fake.App,
			URL:         fake.URL(),
			APIKey:      harness.StarrAPIKey,
			DeleteDelay: time.Second,
		}},
	}
	if tweak != nil {
		tweak(&opts)
	}

	return harness.Start(t, opts)
}

func waitStarrPoll(t *testing.T, fake *starrfake.Server) {
	t.Helper()

	deadline := time.Now().Add(harness.ExtractTimeout)
	for time.Now().Before(deadline) {
		if fake.QueueGets() >= 1 {
			// retrieveAppQueues runs checkStarrQueue after the HTTP GET returns.
			time.Sleep(100 * time.Millisecond)

			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("unpackerr never polled the fake Starr queue")
}

func assertExtractedMarker(t *testing.T, root string) {
	t.Helper()

	got, ok, err := fixtures.WalkReadBase(root, fixtures.MarkerName)
	if err != nil {
		t.Fatal(err)
	}

	if !ok {
		t.Fatalf("extracted %s not found under %s", fixtures.MarkerName, root)
	}

	if got != fixtures.Marker {
		t.Fatalf("marker %q", got)
	}
}

func TestStarrDownloadingNeverExtracts(t *testing.T) {
	t.Parallel()

	fake := newFake(t, starrfake.AppSonarr)
	out, src := fixtures.DownloadSet(t, t.TempDir(), "DL")
	fixtures.RAR(t, filepath.Join(out, "show.rar"), src, fixtures.RAROptions{})

	title := fixtures.SceneName("DL")
	fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusDownloading,
		OutputPath: out,
		Protocol:   starrfake.ProtocolTorrent,
	})

	h := startStarr(t, fake, nil)
	waitStarrPoll(t, fake)
	h.AssertNeverQueue(t, title, harness.SkipTimeout)
}

func TestStarrFlipCompletedRARImportDelete(t *testing.T) {
	t.Parallel()

	fake := newFake(t, starrfake.AppSonarr)
	out, src := fixtures.DownloadSet(t, t.TempDir(), "FLIP")
	fixtures.RAR(t, filepath.Join(out, "show.rar"), src, fixtures.RAROptions{})

	title := fixtures.SceneName("FLIP")
	rec := fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusDownloading,
		OutputPath: out,
		Protocol:   starrfake.ProtocolTorrent,
	})

	h := startStarr(t, fake, nil)
	waitStarrPoll(t, fake)
	h.AssertNeverQueue(t, title, 2*time.Second)

	if !fake.Complete(rec.ID) {
		t.Fatal("complete")
	}

	h.WaitQueue(t, title, "extracted", harness.ExtractTimeout)
	assertExtractedMarker(t, out)

	if !fake.Drop(rec.ID) {
		t.Fatal("drop")
	}

	h.WaitQueue(t, title, "imported", harness.ExtractTimeout)
	h.WaitHistory(t, title, "imported", harness.ExtractTimeout)
	h.WaitHistory(t, title, "deleted", harness.ExtractTimeout)
}

func TestStarrCompletedZip(t *testing.T) {
	t.Parallel()

	fake := newFake(t, starrfake.AppSonarr)
	out, src := fixtures.DownloadSet(t, t.TempDir(), "ZIP")
	fixtures.ZIP(t, filepath.Join(out, "show.zip"), src, "")

	title := fixtures.SceneName("ZIP")
	fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusCompleted,
		OutputPath: out,
	})

	h := startStarr(t, fake, nil)
	h.WaitQueue(t, title, "extracted", harness.ExtractTimeout)
	assertExtractedMarker(t, out)
}

func TestStarrCompleted7z(t *testing.T) {
	t.Parallel()

	fixtures.Require7z(t)

	fake := newFake(t, starrfake.AppSonarr)
	out, src := fixtures.DownloadSet(t, t.TempDir(), "SEVEN")
	fixtures.SevenZipArchive(t, filepath.Join(out, "show.7z"), src, "")

	title := fixtures.SceneName("SEVEN")
	fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusCompleted,
		OutputPath: out,
	})

	h := startStarr(t, fake, nil)
	h.WaitQueue(t, title, "extracted", harness.ExtractTimeout)
	assertExtractedMarker(t, out)
}

func TestStarrPasswordFromConfig(t *testing.T) {
	t.Parallel()

	fake := newFake(t, starrfake.AppSonarr)
	out, src := fixtures.DownloadSet(t, t.TempDir(), "PWC")
	fixtures.RAR(t, filepath.Join(out, "show.rar"), src, fixtures.RAROptions{Password: harness.Password})

	title := fixtures.SceneName("PWC")
	fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusCompleted,
		OutputPath: out,
	})

	h := startStarr(t, fake, func(opts *harness.Options) {
		opts.Passwords = []string{harness.Password}
	})
	h.WaitQueue(t, title, "extracted", harness.ExtractTimeout)
	assertExtractedMarker(t, out)
}

func TestStarrPasswordFromPath(t *testing.T) {
	t.Parallel()

	fake := newFake(t, starrfake.AppSonarr)
	parent := t.TempDir()
	out := filepath.Join(parent, "Show.{{"+harness.Password+"}}")
	src := fixtures.DirWithPayload(t, parent, "pw-src")

	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}

	fixtures.RAR(t, filepath.Join(out, "show.rar"), src, fixtures.RAROptions{Password: harness.Password})

	title := fixtures.SceneName("PWP")
	fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusCompleted,
		OutputPath: out,
	})

	h := startStarr(t, fake, nil)
	h.WaitQueue(t, title, "extracted", harness.ExtractTimeout)
	assertExtractedMarker(t, out)
}

func TestStarrNoArchivesStaysWaiting(t *testing.T) {
	t.Parallel()

	fake := newFake(t, starrfake.AppSonarr)
	out := filepath.Join(t.TempDir(), fixtures.SceneName("EMPTY"))
	if err := fixtures.MustPayload(out); err != nil {
		t.Fatal(err)
	}

	title := fixtures.SceneName("EMPTY")
	fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusCompleted,
		OutputPath: out,
	})

	h := startStarr(t, fake, nil)
	h.WaitQueue(t, title, "waiting", harness.ExtractTimeout)
	h.AssertStatusNot(t, title, "extracted", 20*time.Second)
}

func TestStarrProtocolSkip(t *testing.T) {
	t.Parallel()

	fake := newFake(t, starrfake.AppSonarr)
	out, src := fixtures.DownloadSet(t, t.TempDir(), "NZB")
	fixtures.RAR(t, filepath.Join(out, "show.rar"), src, fixtures.RAROptions{})

	title := fixtures.SceneName("NZB")
	fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusCompleted,
		OutputPath: out,
		Protocol:   starrfake.ProtocolUsenet,
	})

	h := startStarr(t, fake, func(opts *harness.Options) {
		opts.Starr[0].Protocols = "torrent,TorrentDownloadProtocol"
	})
	waitStarrPoll(t, fake)
	h.AssertNeverQueue(t, title, harness.SkipTimeout)
}

func TestStarrSyncthingTmp(t *testing.T) {
	t.Parallel()

	fake := newFake(t, starrfake.AppSonarr)
	out, src := fixtures.DownloadSet(t, t.TempDir(), "SYNC")
	fixtures.RAR(t, filepath.Join(out, "show.rar"), src, fixtures.RAROptions{})

	tmp := filepath.Join(out, "file.tmp")
	if err := os.WriteFile(tmp, []byte("syncing"), 0o644); err != nil {
		t.Fatal(err)
	}

	title := fixtures.SceneName("SYNC")
	fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusCompleted,
		OutputPath: out,
	})

	h := startStarr(t, fake, func(opts *harness.Options) {
		opts.Starr[0].Syncthing = true
	})
	h.WaitQueue(t, title, "waiting", harness.ExtractTimeout)
	h.AssertStatusNot(t, title, "extracted", 18*time.Second)

	if err := os.Remove(tmp); err != nil {
		t.Fatal(err)
	}

	h.WaitQueue(t, title, "extracted", harness.ExtractTimeout)
	assertExtractedMarker(t, out)
}

func TestStarrLidarrZip(t *testing.T) {
	t.Parallel()

	fake := newFake(t, starrfake.AppLidarr)
	out, src := fixtures.DownloadSet(t, t.TempDir(), "LID")
	fixtures.ZIP(t, filepath.Join(out, "album.zip"), src, "")

	title := fixtures.SceneName("LID")
	fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusCompleted,
		OutputPath: out,
		ArtistID:   4,
		AlbumID:    8,
	})

	h := startStarr(t, fake, nil)
	h.WaitQueue(t, title, "extracted", harness.ExtractTimeout)
	assertExtractedMarker(t, out)
}

func TestStarrMultiVolumePartRAR(t *testing.T) {
	t.Parallel()

	fake := newFake(t, starrfake.AppSonarr)
	out, src := fixtures.DownloadSet(t, t.TempDir(), "VOL")
	fixtures.RAR(t, filepath.Join(out, "show.rar"), src, fixtures.RAROptions{Volume: "2k"})

	title := fixtures.SceneName("VOL")
	fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusCompleted,
		OutputPath: out,
	})

	h := startStarr(t, fake, nil)
	h.WaitQueue(t, title, "extracted", harness.ExtractTimeout)
	assertExtractedMarker(t, out)
}

func TestStarrRARinZIP(t *testing.T) {
	t.Parallel()

	fake := newFake(t, starrfake.AppSonarr)
	out, src := fixtures.DownloadSet(t, t.TempDir(), "RIZ")
	fixtures.RARinZIP(t, filepath.Join(out, "show.zip"), src)

	title := fixtures.SceneName("RIZ")
	fake.Add(starrfake.Record{
		Title:      title,
		Status:     starrfake.StatusCompleted,
		OutputPath: out,
	})

	h := startStarr(t, fake, nil)
	h.WaitQueue(t, title, "extracted", harness.ExtractTimeout)
	assertExtractedMarker(t, out)
}
