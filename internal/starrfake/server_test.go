package starrfake_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/Unpackerr/unpackerr-inttest/internal/starrfake"
)

func TestQueueAuthAndPagination(t *testing.T) {
	t.Parallel()

	fake := starrfake.New(starrfake.AppSonarr, "secret-key-not-used-for-length")
	url := fake.Start()
	t.Cleanup(fake.Close)

	for i := 1; i <= 5; i++ {
		fake.Add(starrfake.Record{Title: "item-" + strconv.Itoa(i), Status: starrfake.StatusCompleted})
	}

	req, err := http.NewRequest(http.MethodGet, url+"/api/v3/queue?page=2&pageSize=2", nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing key: status %d", res.StatusCode)
	}

	req.Header.Set("X-Api-Key", "wrong")

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong key: status %d", res.StatusCode)
	}

	req.Header.Set("X-Api-Key", "secret-key-not-used-for-length")

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", res.StatusCode, body)
	}

	var queue starrfake.Queue
	if err := json.Unmarshal(body, &queue); err != nil {
		t.Fatal(err)
	}

	if queue.TotalRecords != 5 || queue.Page != 2 || queue.PageSize != 2 {
		t.Fatalf("page meta %+v", queue)
	}

	if len(queue.Records) != 2 || queue.Records[0].Title != "item-3" || queue.Records[1].Title != "item-4" {
		t.Fatalf("records %+v", queue.Records)
	}
}

func TestLidarrV1AndMutators(t *testing.T) {
	t.Parallel()

	fake := starrfake.New(starrfake.AppLidarr, "lidarr-key")
	url := fake.Start()
	t.Cleanup(fake.Close)

	rec := fake.Add(starrfake.Record{
		Title:      "Album",
		Status:     starrfake.StatusDownloading,
		OutputPath: "/dl/Album",
		ArtistID:   9,
	})

	if rec.Sizeleft != rec.Size {
		t.Fatalf("downloading sizeleft %v", rec.Sizeleft)
	}

	if !fake.Complete(rec.ID) {
		t.Fatal("complete")
	}

	req, err := http.NewRequest(http.MethodGet, url+"/api/v1/queue?page=1&pageSize=2000", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("X-Api-Key", "lidarr-key")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()

	var queue starrfake.Queue
	if err := json.NewDecoder(res.Body).Decode(&queue); err != nil {
		t.Fatal(err)
	}

	if queue.TotalRecords != 1 || queue.Records[0].Status != starrfake.StatusCompleted {
		t.Fatalf("queue %+v", queue)
	}

	if queue.Records[0].Sizeleft != 0 || queue.Records[0].ArtistID != 9 {
		t.Fatalf("record %+v", queue.Records[0])
	}

	if !fake.Drop(rec.ID) {
		t.Fatal("drop")
	}

	if n := len(fake.Snapshot()); n != 0 {
		t.Fatalf("left %d", n)
	}

	req, err = http.NewRequest(http.MethodGet, url+"/api/v3/queue", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("X-Api-Key", "lidarr-key")

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("sonarr path on lidarr: %d", res.StatusCode)
	}
}

func TestQueueHugePageDoesNotPanic(t *testing.T) {
	t.Parallel()

	fake := starrfake.New(starrfake.AppSonarr, "secret-key-not-used-for-length")
	url := fake.Start()
	t.Cleanup(fake.Close)
	fake.Add(starrfake.Record{Title: "one", Status: starrfake.StatusCompleted})

	req, err := http.NewRequest(http.MethodGet,
		url+"/api/v3/queue?page=9223372036854775807&pageSize=9223372036854775807", nil)
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
		t.Fatalf("status %d", res.StatusCode)
	}

	var queue starrfake.Queue
	if err := json.NewDecoder(res.Body).Decode(&queue); err != nil {
		t.Fatal(err)
	}

	if queue.TotalRecords != 1 || len(queue.Records) != 0 {
		t.Fatalf("overflow page should be empty, got %+v", queue)
	}
}
