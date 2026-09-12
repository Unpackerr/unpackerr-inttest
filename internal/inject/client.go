package inject

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Unpackerr/unpackerr-inttest/internal/starrfake"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Add POSTs a queue row to faker's /{app}/debug/add.
func Add(fakerURL, app string, rec starrfake.Record) (starrfake.Record, error) {
	body, err := json.Marshal(rec)
	if err != nil {
		return starrfake.Record{}, err
	}

	url := fakerURL + "/" + app + "/debug/add"

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return starrfake.Record{}, err
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := httpClient.Do(req)
	if err != nil {
		return starrfake.Record{}, fmt.Errorf("POST %s: %w", url, err)
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return starrfake.Record{}, err
	}

	if res.StatusCode != http.StatusCreated {
		return starrfake.Record{}, fmt.Errorf("POST %s: %s: %s", url, res.Status, raw)
	}

	var out starrfake.Record
	if err := json.Unmarshal(raw, &out); err != nil {
		return starrfake.Record{}, fmt.Errorf("decode add: %w", err)
	}

	return out, nil
}

func decorate(app string, rec starrfake.Record) starrfake.Record {
	switch app {
	case starrfake.AppSonarr:
		if rec.SeriesID == 0 {
			rec.SeriesID = 1
		}

		if rec.EpisodeID == 0 {
			rec.EpisodeID = 1
		}
	case starrfake.AppRadarr:
		if rec.MovieID == 0 {
			rec.MovieID = 1
		}
	case starrfake.AppLidarr:
		if rec.ArtistID == 0 {
			rec.ArtistID = 1
		}

		if rec.AlbumID == 0 {
			rec.AlbumID = 1
		}
	case starrfake.AppReadarr:
		if rec.AuthorID == 0 {
			rec.AuthorID = 1
		}

		if rec.BookID == 0 {
			rec.BookID = 1
		}
	}

	return rec
}
