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
make test                               # go test -tags=integration -timeout 5m ./test/...
UNPACKERR_BIN=/usr/local/bin/unpackerr make test
```

Timers in the generated TOML are short (`interval` / `start_delay` around
200ms–1s). Folder poll is off (`interval = "0s"`) unless a test sets it.
Current Unpackerr still floors Starr `interval` and `start_delay` at 15s, so
extract assertions wait up to a minute. Restart tests reuse the same config
dir so `unpackerr.history.jsonl` can restore the live queue.

## Play along (watch the UI)

CI tests spawn Unpackerr. For a manual session you run two daemons yourself
and inject queue rows:

1. Fake Starr (all four apps on one port):

   ```bash
   go run ./cmd/faker -listen 127.0.0.1:8989
   # or: make faker && ./faker
   ```

   Routes are `/sonarr`, `/radarr`, `/lidarr`, `/readarr`. Point Unpackerr at:

   ```toml
   [[sonarr]]
   url = "http://127.0.0.1:8989/sonarr"
   api_key = "unpackerr-inttest-starr-key-32ch"

   [[radarr]]
   url = "http://127.0.0.1:8989/radarr"
   api_key = "unpackerr-inttest-starr-key-32ch"

   [[lidarr]]
   url = "http://127.0.0.1:8989/lidarr"
   api_key = "unpackerr-inttest-starr-key-32ch"

   [[readarr]]
   url = "http://127.0.0.1:8989/readarr"
   api_key = "unpackerr-inttest-starr-key-32ch"
   ```

2. Start Unpackerr yourself (GUI, `make dev`, whatever) and watch the UI.

3. Inject items. Either stage a built-in layout, or point at a folder you
   already have:

   ```bash
   make inject
   ./inject -a sonarr -b rar
   ./inject -a all -b zip -size 32m -compress   # four apps, slower extract
   ./inject -a sonarr -t Some.Show.S01E01 -p /abs/path/to/download
   ```

   `-p` and `-t` are mutually exclusive with `-b`. `-a` accepts `sonarr`,
   `radarr`, `lidarr`, `readarr`, a comma list, or `all`. `-b` writes under
   `-d` (default `./downloads`). `-size` is the payload (`4k`, `32m`, `1g`);
   `-compress` uses real compression so the progress meter has something to
   do. `-status downloading` leaves the row transferring until you:

   ```bash
   curl -sS -X POST http://127.0.0.1:8989/sonarr/debug/complete/1
   curl -sS -X POST http://127.0.0.1:8989/sonarr/debug/drop/1
   ```

   Built-ins: `rar`, `zip`, `7z`, `recursive` (zip-in-zip), `nests` (three
   inner zips), `subs` (archive in `Season.01/`), `depth` (archives at
   different folder depths), `volumes`, `r00`, `password` (folder name
   `{{hunter2}}`), `empty`, `rarzip`, `syncthing` (`.tmp` sibling).

## Layout

- `internal/starrfake` — in-memory queue, pagination, `X-Api-Key`, four-app hub
- `internal/fixtures` — NFO/txt + urandom payloads; rar/zip/7z at test start
- `internal/harness` — named `[folder.*]` / `[sonarr.*]` TOML, restart, `/api/queue` retry
- `internal/httpcap` — records webhook JSON (`unpackerr_eventtype`, `data`, titles)
- `internal/write` — slow chunked writer for folder `start_delay`
- `internal/inject` — CLI that stages patterns and POSTs `/debug/add`
- `cmd/faker` — long-running fake Starr (`/{app}/api/...`)
- `cmd/inject` — feed faker without spawning Unpackerr
- `test/` — `//go:build integration`

Do not commit binary archives. GitHub Actions installs `rar` / `p7zip-full` /
`zip`, checks out Unpackerr (dispatch `unpackerr_ref`, then `UNPACKERR_REF`,
then `main`), builds it, and runs the integration tag. A manual run accepts a
branch, tag, or commit SHA.
