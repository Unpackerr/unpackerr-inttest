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

func TestHubIndexAPIKeyOnlyOnLoopback(t *testing.T) {
	t.Parallel()

	hub := starrfake.NewHub("the-secret-key-value")
	index := func() map[string]any {
		t.Helper()

		srv := httptest.NewServer(hub.Handler())
		t.Cleanup(srv.Close)

		res, err := http.Get(srv.URL + "/")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()

		var body map[string]any
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}

		return body
	}

	hub.Listen = "0.0.0.0:8989"
	if _, ok := index()["apiKey"]; ok {
		t.Fatal("expected no apiKey when listening on all interfaces")
	}

	hub.Listen = ":8989"
	if _, ok := index()["apiKey"]; ok {
		t.Fatal("expected no apiKey when listening on :port")
	}

	hub.Listen = "127.0.0.1:8989"
	if got, _ := index()["apiKey"].(string); got != "the-secret-key-value" {
		t.Fatalf("loopback apiKey %v", got)
	}

	hub.Listen = "[::1]:8989"
	if got, _ := index()["apiKey"].(string); got != "the-secret-key-value" {
		t.Fatalf("ipv6 loopback apiKey %v", got)
	}
}
