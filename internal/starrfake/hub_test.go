package starrfake_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Unpackerr/unpackerr-inttest/internal/starrfake"
)

func TestHubURLBases(t *testing.T) {
	t.Parallel()

	hub := starrfake.NewHub("secret-key-not-used-for-length")
	srv := httptest.NewServer(hub.Handler())
	t.Cleanup(srv.Close)

	added := hub.App(starrfake.AppSonarr).Add(starrfake.Record{
		Title:      "Show",
		Status:     starrfake.StatusCompleted,
		OutputPath: "/dl/Show",
	})
	if added.ID == 0 {
		t.Fatal("id")
	}

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/sonarr/api/v3/queue?page=1&pageSize=10", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("X-Api-Key", "secret-key-not-used-for-length")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("sonarr queue %d: %s", res.StatusCode, body)
	}

	var queue starrfake.Queue
	if err := json.NewDecoder(res.Body).Decode(&queue); err != nil {
		t.Fatal(err)
	}

	if queue.TotalRecords != 1 || queue.Records[0].Title != "Show" {
		t.Fatalf("sonarr queue %+v", queue)
	}

	req, err = http.NewRequest(http.MethodGet, srv.URL+"/lidarr/api/v1/queue", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("X-Api-Key", "secret-key-not-used-for-length")

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if err := json.NewDecoder(res.Body).Decode(&queue); err != nil {
		t.Fatal(err)
	}

	if queue.TotalRecords != 0 {
		t.Fatalf("lidarr should be empty: %+v", queue)
	}

	res, err = http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), `"app":"radarr"`) {
		t.Fatalf("index %d: %s", res.StatusCode, body)
	}
}

func TestHubDebugAddPrefixed(t *testing.T) {
	t.Parallel()

	hub := starrfake.NewHub("k")
	srv := httptest.NewServer(hub.Handler())
	t.Cleanup(srv.Close)

	res, err := http.Post(srv.URL+"/radarr/debug/add", "application/json",
		strings.NewReader(`{"title":"Movie","status":"completed","outputPath":"/dl/Movie"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("add %d: %s", res.StatusCode, body)
	}

	snap := hub.App(starrfake.AppRadarr).Snapshot()
	if len(snap) != 1 || snap[0].Title != "Movie" {
		t.Fatalf("radarr snapshot %+v", snap)
	}

	if n := len(hub.App(starrfake.AppSonarr).Snapshot()); n != 0 {
		t.Fatalf("sonarr leaked %d", n)
	}
}
