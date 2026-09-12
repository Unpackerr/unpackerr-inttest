package starrfake

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// Apps is the four Starr apps Unpackerr polls.
func Apps() []string {
	return []string{AppSonarr, AppRadarr, AppLidarr, AppReadarr}
}

// ValidApp reports whether name is sonarr, radarr, lidarr, or readarr.
func ValidApp(name string) bool {
	switch strings.ToLower(name) {
	case AppSonarr, AppRadarr, AppLidarr, AppReadarr:
		return true
	default:
		return false
	}
}

// Hub serves all four fake Starr queues on URL bases: /sonarr, /radarr, /lidarr, /readarr.
type Hub struct {
	Key    string
	Listen string // bind address; GET / includes apiKey only when this is loopback
	apps   map[string]*Server
}

// NewHub starts four isolated in-memory queues sharing apiKey.
func NewHub(apiKey string) *Hub {
	hub := &Hub{
		Key:  apiKey,
		apps: make(map[string]*Server, 4),
	}

	for _, app := range Apps() {
		hub.apps[app] = New(app, apiKey)
	}

	return hub
}

// App returns the fake for name, or nil.
func (h *Hub) App(name string) *Server {
	return h.apps[strings.ToLower(name)]
}

// Handler mounts each app at /{app}/ and a JSON index at /.
func (h *Hub) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.serveIndex)

	for _, app := range Apps() {
		srv := h.apps[app]
		mux.Handle("/"+app+"/", http.StripPrefix("/"+app, srv.Handler()))
	}

	return mux
}

// ListenAndServe blocks on addr (cmd/faker).
func (h *Hub) ListenAndServe(addr string) error {
	h.Listen = addr

	return http.ListenAndServe(addr, h.Handler()) //nolint:gosec
}

// publishAPIKey is true when faker is bound to loopback (127.0.0.1, ::1, localhost).
func publishAPIKey(listen string) bool {
	if listen == "" {
		return false
	}

	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		host = listen
	}

	if host == "" {
		return false
	}

	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}

func (h *Hub) serveIndex(writer http.ResponseWriter, request *http.Request) {
	type appInfo struct {
		App     string `json:"app"`
		Base    string `json:"base"`
		Queue   string `json:"queue"`
		Add     string `json:"debugAdd"`
		API     string `json:"api"`
		Records int    `json:"records"`
	}

	host := request.Host
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}

	out := make([]appInfo, 0, len(h.apps))
	for _, app := range Apps() {
		srv := h.apps[app]
		base := fmt.Sprintf("%s://%s/%s", scheme, host, app)
		out = append(out, appInfo{
			App:     app,
			Base:    base,
			Queue:   base + "/api/" + srv.APIVersion() + "/queue",
			Add:     base + "/debug/add",
			API:     srv.APIVersion(),
			Records: len(srv.Snapshot()),
		})
	}

	writer.Header().Set("Content-Type", "application/json")

	body := map[string]any{"apps": out}
	if publishAPIKey(h.Listen) {
		body["apiKey"] = h.Key
	}

	_ = json.NewEncoder(writer).Encode(body)
}
