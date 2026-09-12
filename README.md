# unpackerr-inttest

Integration harness for [Unpackerr](https://github.com/Unpackerr/unpackerr).
Unpackerr stays a binary this repo starts. Tests fake Starr queue APIs, build
small archives at runtime, and assert extracts through Unpackerr's HTTP API
and the filesystem.

```
go_test  →  harness  →  fake Starr  (GET /api/v3/queue or /api/v1/queue)
                 ↘     unpackerr binary  →  download dirs / folder watches
                       GET /api/queue  GET /api/history
```

## Requirements

- Go 1.27+
- Host tools: `rar`, `zip`/`unzip`, and `7z` (or `7zz`/`7za`). A missing tool
  skips only the tests that need it.
- An Unpackerr binary:
  - `UNPACKERR_BIN=/path/to/unpackerr`, or
  - a sibling checkout at `../unpackerr` (this repo runs `go build` there)

## Run

```bash
make test-unit                          # faker/fixture/harness unit tests
make test                               # go test -tags=integration -timeout 3m ./test/...
UNPACKERR_BIN=/usr/local/bin/unpackerr make test
```

Timers in the generated TOML are short (`interval` / `start_delay` around
200ms–1s, folder poll 200ms). Current Unpackerr still floors Starr
`interval` and `start_delay` at 15s, so extract assertions wait up to a
minute.

## Layout

- `internal/starrfake` — in-memory queue, pagination, `X-Api-Key`, four-app hub
- `internal/fixtures` — NFO/txt + urandom payloads; rar/zip/7z at test start
- `internal/harness` — temp dir, TOML, exec unpackerr, poll `/api/queue` and `/api/history`
- `internal/write` — slow chunked writer for folder `start_delay`
- `cmd/faker` — one listener for `/sonarr` `/radarr` `/lidarr` `/readarr`
- `test/` — `//go:build integration`

```bash
go run ./cmd/faker -listen 127.0.0.1:8989
# unpackerr [[sonarr]] url = "http://127.0.0.1:8989/sonarr"  (same key, /radarr /lidarr /readarr)
```

Do not commit binary archives. GitHub Actions installs `rar` / `p7zip-full` /
`zip`, checks out Unpackerr (`UNPACKERR_REF`, default `main`), builds it, and
runs the integration tag.
