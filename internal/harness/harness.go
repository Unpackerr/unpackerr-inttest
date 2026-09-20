// Package harness starts a real unpackerr process against generated config.
package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// DefaultAPIKey is a 64-character admin key (Unpackerr requires 60–150).
	DefaultAPIKey = "unpackerr-inttest-admin-key-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	// StarrAPIKey is a 32-character Starr key (Unpackerr requires ≥32).
	StarrAPIKey = "unpackerr-inttest-starr-key-32ch"
	// Password is used for encrypted RAR and 7z tests. Unpackerr/xtractr cannot
	// decrypt zip; do not expect a zip -P archive to extract.
	Password = "hunter2"
	// ReadyTimeout is how long Start waits for /api/system.
	ReadyTimeout = 15 * time.Second
	// ExtractTimeout covers Unpackerr's 15s interval/start_delay floors plus extract.
	ExtractTimeout = 60 * time.Second
	// SkipTimeout is long enough to prove a download never queued.
	SkipTimeout = 8 * time.Second
)

// Starr describes one fake Starr instance in the generated TOML.
type Starr struct {
	Key         string
	App         string
	URL         string
	APIKey      string
	Protocols   string
	DeleteOrig  bool
	DeleteDelay time.Duration
	Syncthing   bool
	Path        string // toml `path`; Unpackerr copies this into `paths`
	Timeout     time.Duration
}

// Folder describes one [folder.<key>] watch path.
type Folder struct {
	Key              string
	Path             string
	Interval         time.Duration
	ExtractPath      string
	ExcludePaths     []string
	WaitExtensions   []string
	SkipEmpty        bool
	DisableRecursion bool
	MaxNested        int
	ExtrasMaxDepth   int
	ExtractISOs      bool
	MoveBack         bool
	DeleteAfter      time.Duration
	DeleteOrig       bool
	DeleteFiles      bool
}

// Webhook describes one [webhook.<key>] destination.
type Webhook struct {
	Key    string
	URL    string
	Events []int
}

// Options control the generated unpackerr.conf.
type Options struct {
	Debug       bool // always forced on; failed tests dump unpackerr logs
	Interval    time.Duration
	StartDelay  time.Duration
	DeleteDelay time.Duration
	RetryDelay  time.Duration
	Progress    time.Duration
	LogQueues   time.Duration
	Passwords   []string
	Starr       []Starr
	Folders     []Folder
	Webhooks    []Webhook
	HookIDs     map[string]string
	HookTitles  map[string]string
	KeepHistory uint
	Binary      string
}

func defaultOptions(opts Options) Options {
	if opts.Interval == 0 {
		opts.Interval = 200 * time.Millisecond
	}

	if opts.StartDelay == 0 {
		opts.StartDelay = time.Second
	}

	if opts.DeleteDelay == 0 {
		opts.DeleteDelay = time.Second
	}

	if opts.RetryDelay == 0 {
		opts.RetryDelay = time.Minute
	}

	if opts.Progress == 0 {
		opts.Progress = time.Second
	}

	if opts.LogQueues == 0 {
		opts.LogQueues = 200 * time.Millisecond
	}

	if opts.KeepHistory == 0 {
		opts.KeepHistory = 50
	}

	// Always on so t.Failed dumps unpackerr stdout. Options.Debug is not a caller knob.
	opts.Debug = true

	return opts
}

// H is a running unpackerr plus temp dir and log buffer.
type H struct {
	Dir     string
	Config  string
	LogFile string
	Addr    string
	APIKey  string
	Client  *http.Client
	bin     string
	cmd     *exec.Cmd
	logs    *bytes.Buffer
	logMu   sync.Mutex
	opts    Options
}

// Start writes config, execs unpackerr, and waits until the HTTP API answers.
func Start(t *testing.T, opts Options) *H {
	t.Helper()

	opts = defaultOptions(opts)
	bin, err := EnsureBinary(opts.Binary)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "set UNPACKERR_BIN") {
			t.Skip(err.Error())
		}

		t.Fatal(err)
	}

	dir := t.TempDir()
	addr, err := freeAddr()
	if err != nil {
		t.Fatal(err)
	}

	h := &H{
		Dir:     dir,
		Config:  filepath.Join(dir, "unpackerr.conf"),
		LogFile: filepath.Join(dir, "unpackerr.log"),
		Addr:    addr,
		APIKey:  DefaultAPIKey,
		Client:  &http.Client{Timeout: 5 * time.Second},
		bin:     bin,
		logs:    new(bytes.Buffer),
		opts:    opts,
	}

	if err := os.WriteFile(h.Config, []byte(renderConfig(h, opts)), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		h.Stop()
		if t.Failed() {
			t.Logf("unpackerr logs:\n%s", h.Logs())
			if data, err := os.ReadFile(h.LogFile); err == nil {
				t.Logf("unpackerr.log:\n%s", data)
			}
		}
	})

	h.launch(t)

	return h
}

func (h *H) launch(t *testing.T) {
	t.Helper()

	cmd := exec.Command(h.bin, "-c", h.Config)
	cmd.Dir = h.Dir
	cmd.Env = []string{
		"HOME=" + h.Dir,
		"PATH=" + os.Getenv("PATH"),
		"TMPDIR=" + h.Dir,
		"USER=" + os.Getenv("USER"),
		"LANG=C",
	}
	prepareCmd(cmd)
	cmd.Stdout = &logWriter{h: h}
	cmd.Stderr = &logWriter{h: h}
	h.cmd = cmd

	if err := cmd.Start(); err != nil {
		t.Fatalf("start unpackerr: %v", err)
	}

	if err := h.Ready(ReadyTimeout); err != nil {
		t.Fatalf("unpackerr not ready: %v\n%s", err, h.Logs())
	}
}

// WriteConfig replaces unpackerr.conf. Call Restart to pick up folder/Starr changes.
func (h *H) WriteConfig(t *testing.T, opts Options) {
	t.Helper()

	opts = defaultOptions(opts)
	opts.Binary = h.opts.Binary
	h.opts = opts

	if err := os.WriteFile(h.Config, []byte(renderConfig(h, opts)), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Restart stops unpackerr and starts it again with the same config directory
// so history JSONL can restore the live queue.
func (h *H) Restart(t *testing.T) {
	t.Helper()

	h.Stop()
	time.Sleep(150 * time.Millisecond)
	h.launch(t)
}

// Stop sends SIGTERM to the process group, then SIGKILL.
func (h *H) Stop() {
	if h == nil || h.cmd == nil {
		return
	}

	stopCmd(h.cmd)
	h.cmd = nil
}

// Logs is the captured stdout/stderr so far.
func (h *H) Logs() string {
	h.logMu.Lock()
	defer h.logMu.Unlock()

	return h.logs.String()
}

// BaseURL is the unpackerr HTTP root.
func (h *H) BaseURL() string {
	return "http://" + h.Addr
}

type logWriter struct {
	h *H
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.h.logMu.Lock()
	defer w.h.logMu.Unlock()
	return w.h.logs.Write(p)
}

func freeAddr() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen: %w", err)
	}

	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		return "", fmt.Errorf("close: %w", err)
	}

	return addr, nil
}

func (h *H) get(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, h.BaseURL()+path, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", h.APIKey)

	return h.Client.Do(req)
}

func (h *H) post(path, body string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodPost, h.BaseURL()+path, strings.NewReader(body))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("X-Api-Key", h.APIKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := h.Client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = res.Body.Close() }()

	payload, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, res.StatusCode, err
	}

	return payload, res.StatusCode, nil
}

// Retry POSTs /api/queue/retry for the queue row id (usually the path).
func (h *H) Retry(t *testing.T, id string) {
	t.Helper()

	body, code, err := h.post("/api/queue/retry", fmt.Sprintf(`{"id":%q}`, id))
	if err != nil {
		t.Fatal(err)
	}

	if code != http.StatusOK {
		t.Fatalf("retry %s: status %d %s", id, code, body)
	}
}

// Ready polls GET /api/system until it returns 200.
func (h *H) Ready(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	var last error
	for time.Now().Before(deadline) {
		res, err := h.get("/api/system")
		if err == nil {
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()

			if res.StatusCode == http.StatusOK {
				return nil
			}

			last = fmt.Errorf("status %d", res.StatusCode)
		} else {
			last = err
		}

		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("ready: %w", last)
}

// QueueItem is GET /api/queue.
type QueueItem struct {
	ID      string    `json:"id"`
	App     string    `json:"app"`
	Path    string    `json:"path"`
	Status  string    `json:"status"`
	Error   string    `json:"error"`
	Note    string    `json:"note"`
	Event   string    `json:"event"`
	DueKind string    `json:"dueKind"`
	Due     time.Time `json:"due"`
	Archive string    `json:"archive"`
}

// HistoryRecord is GET /api/history.
type HistoryRecord struct {
	ID     string `json:"id"`
	App    string `json:"app"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

// Queue returns the live extract queue.
func (h *H) Queue() ([]QueueItem, error) {
	res, err := h.get("/api/queue")
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("queue status %d: %s", res.StatusCode, body)
	}

	var items []QueueItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("queue json: %w (%s)", err, body)
	}

	return items, nil
}

// History returns durable history rows.
func (h *H) History() ([]HistoryRecord, error) {
	res, err := h.get("/api/history")
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("history status %d: %s", res.StatusCode, body)
	}

	var items []HistoryRecord
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("history json: %w (%s)", err, body)
	}

	return items, nil
}

// Stats is GET /api/stats.
type Stats struct {
	StarrQueues []StarrQueueStat `json:"starrQueues"`
}

// StarrQueueStat is one Starr instance's last activity-queue poll.
type StarrQueueStat struct {
	App       string    `json:"app"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	UpdatedAt time.Time `json:"updatedAt"`
	Error     string    `json:"error"`
}

// Stats returns process counters including Starr poll timestamps.
func (h *H) Stats() (*Stats, error) {
	res, err := h.get("/api/stats")
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stats status %d: %s", res.StatusCode, body)
	}

	var stats Stats
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("stats json: %w (%s)", err, body)
	}

	return &stats, nil
}

func matchID(id, want string) bool {
	return id == want || strings.HasSuffix(id, want) || strings.Contains(id, want)
}

// FindQueue returns the first queue row whose id or path matches want.
func (h *H) FindQueue(want string) (QueueItem, bool, error) {
	items, err := h.Queue()
	if err != nil {
		return QueueItem{}, false, err
	}

	for _, item := range items {
		if matchID(item.ID, want) || matchID(item.Path, want) {
			return item, true, nil
		}
	}

	return QueueItem{}, false, nil
}

// WaitQueue waits until a matching queue row has status.
func (h *H) WaitQueue(t *testing.T, want, status string, timeout time.Duration) QueueItem {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var last []QueueItem

	for time.Now().Before(deadline) {
		items, err := h.Queue()
		if err == nil {
			last = items
			for _, item := range items {
				if (matchID(item.ID, want) || matchID(item.Path, want)) && item.Status == status {
					return item
				}
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("timeout waiting for queue %q status %s; last=%+v", want, status, last)

	return QueueItem{}
}

// WaitHistory waits until a matching history row has status.
func (h *H) WaitHistory(t *testing.T, want, status string, timeout time.Duration) HistoryRecord {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var last []HistoryRecord

	for time.Now().Before(deadline) {
		items, err := h.History()
		if err == nil {
			last = items
			for _, item := range items {
				if (matchID(item.ID, want) || matchID(item.Path, want)) && item.Status == status {
					return item
				}
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("timeout waiting for history %q status %s; last=%+v", want, status, last)

	return HistoryRecord{}
}

// AssertNeverQueue waits timeout and fails if a matching row appears.
func (h *H) AssertNeverQueue(t *testing.T, want string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		item, ok, err := h.FindQueue(want)
		if err != nil {
			t.Fatal(err)
		}

		if ok {
			t.Fatalf("did not expect queue item %q: %+v", want, item)
		}

		time.Sleep(200 * time.Millisecond)
	}
}

// AssertStatusNot waits timeout and fails if matching row reaches status.
func (h *H) AssertStatusNot(t *testing.T, want, status string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		item, ok, err := h.FindQueue(want)
		if err != nil {
			t.Fatal(err)
		}

		if ok && item.Status == status {
			t.Fatalf("did not expect %q to become %s: %+v", want, status, item)
		}

		hist, err := h.History()
		if err != nil {
			t.Fatal(err)
		}

		for _, rec := range hist {
			if (matchID(rec.ID, want) || matchID(rec.Path, want)) && rec.Status == status {
				t.Fatalf("did not expect history %q %s: %+v", want, status, rec)
			}
		}

		time.Sleep(200 * time.Millisecond)
	}
}

// WaitQueueFunc waits until a matching queue row satisfies ok.
func (h *H) WaitQueueFunc(t *testing.T, want string, timeout time.Duration, ok func(QueueItem) bool) QueueItem {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var last []QueueItem

	for time.Now().Before(deadline) {
		items, err := h.Queue()
		if err == nil {
			last = items
			for _, item := range items {
				if (matchID(item.ID, want) || matchID(item.Path, want)) && ok(item) {
					return item
				}
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("timeout waiting for queue %q; last=%+v", want, last)

	return QueueItem{}
}

// AssertNeverHistory waits timeout and fails if a matching history row appears.
func (h *H) AssertNeverHistory(t *testing.T, want string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		hist, err := h.History()
		if err != nil {
			t.Fatal(err)
		}

		for _, rec := range hist {
			if matchID(rec.ID, want) || matchID(rec.Path, want) {
				t.Fatalf("did not expect history %q: %+v", want, rec)
			}
		}

		time.Sleep(200 * time.Millisecond)
	}
}
