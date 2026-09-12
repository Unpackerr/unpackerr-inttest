// Package starrfake serves the Starr queue endpoints Unpackerr polls.
package starrfake

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
)

const (
	// AppSonarr is a Sonarr-shaped queue (API v3).
	AppSonarr = "sonarr"
	// AppRadarr is a Radarr-shaped queue (API v3).
	AppRadarr = "radarr"
	// AppLidarr is a Lidarr-shaped queue (API v1).
	AppLidarr = "lidarr"
	// AppReadarr is a Readarr-shaped queue (API v1).
	AppReadarr = "readarr"

	// StatusDownloading is a Starr queue item still transferring.
	StatusDownloading = "downloading"
	// StatusCompleted is a Starr queue item ready to extract.
	StatusCompleted = "completed"

	// ProtocolTorrent is the Starr torrent protocol string.
	ProtocolTorrent = "torrent"
	// ProtocolUsenet is the Starr usenet protocol string.
	ProtocolUsenet = "usenet"
)

// Record is one Starr activity-queue row. Field names match the real APIs.
type Record struct {
	ID         int64   `json:"id"`
	Title      string  `json:"title"`
	Status     string  `json:"status"`
	Protocol   string  `json:"protocol"`
	OutputPath string  `json:"outputPath"`
	Size       float64 `json:"size"`
	Sizeleft   float64 `json:"sizeleft"`
	DownloadID string  `json:"downloadId,omitempty"`
	SeriesID   int64   `json:"seriesId,omitempty"`
	EpisodeID  int64   `json:"episodeId,omitempty"`
	MovieID    int64   `json:"movieId,omitempty"`
	ArtistID   int64   `json:"artistId,omitempty"`
	AlbumID    int64   `json:"albumId,omitempty"`
	AuthorID   int64   `json:"authorId,omitempty"`
	BookID     int64   `json:"bookId,omitempty"`
}

// Queue is the paginated GET /api/{v1,v3}/queue body.
type Queue struct {
	Page          int      `json:"page"`
	PageSize      int      `json:"pageSize"`
	SortKey       string   `json:"sortKey"`
	SortDirection string   `json:"sortDirection"`
	TotalRecords  int      `json:"totalRecords"`
	Records       []Record `json:"records"`
}

// Server is an in-memory Starr queue with an HTTP listener.
type Server struct {
	App string
	Key string

	mu        sync.Mutex
	nextID    int64
	records   []Record
	queueGets int
	http      *httptest.Server
}

// New returns a stopped fake. Call Start or ListenAndServe.
func New(app, apiKey string) *Server {
	if app == "" {
		app = AppSonarr
	}

	return &Server{
		App:    strings.ToLower(app),
		Key:    apiKey,
		nextID: 1,
	}
}

// APIVersion is v3 for Sonarr/Radarr and v1 for Lidarr/Readarr.
func (s *Server) APIVersion() string {
	switch s.App {
	case AppLidarr, AppReadarr:
		return "v1"
	default:
		return "v3"
	}
}

// Handler serves queue GETs (API-key gated) and optional /debug mutators.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	ver := s.APIVersion()
	mux.HandleFunc("GET /api/"+ver+"/queue", s.requireKey(s.serveQueue))
	mux.HandleFunc("POST /debug/add", s.serveAdd)
	mux.HandleFunc("POST /debug/complete/{id}", s.serveComplete)
	mux.HandleFunc("POST /debug/drop/{id}", s.serveDrop)
	mux.HandleFunc("GET /debug/queue", s.serveDebugQueue)

	return mux
}

// Start listens on a random local port (httptest).
func (s *Server) Start() string {
	s.http = httptest.NewServer(s.Handler())

	return s.http.URL
}

// URL is the httptest base URL. Empty until Start.
func (s *Server) URL() string {
	if s.http == nil {
		return ""
	}

	return s.http.URL
}

// Close shuts down the httptest listener.
func (s *Server) Close() {
	if s.http != nil {
		s.http.Close()
		s.http = nil
	}
}

// ListenAndServe blocks on addr (cmd/faker).
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.Handler()) //nolint:gosec
}

// Add appends a record, filling id/size defaults. Status defaults to downloading.
func (s *Server) Add(rec Record) Record {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rec.ID == 0 {
		rec.ID = s.nextID
		s.nextID++
	} else if rec.ID >= s.nextID {
		s.nextID = rec.ID + 1
	}

	if rec.Status == "" {
		rec.Status = StatusDownloading
	}

	if rec.Protocol == "" {
		rec.Protocol = ProtocolTorrent
	}

	if rec.Size == 0 {
		rec.Size = 1
	}

	if rec.Status != StatusCompleted && rec.Sizeleft == 0 {
		rec.Sizeleft = rec.Size
	}

	if rec.Status == StatusCompleted {
		rec.Sizeleft = 0
	}

	if rec.DownloadID == "" {
		rec.DownloadID = rec.Title
	}

	s.records = append(s.records, rec)

	return rec
}

// Complete marks the record completed (sizeleft 0).
func (s *Server) Complete(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.records {
		if s.records[i].ID == id {
			s.records[i].Status = StatusCompleted
			s.records[i].Sizeleft = 0

			return true
		}
	}

	return false
}

// Drop removes the record (Starr imported it).
func (s *Server) Drop(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.records {
		if s.records[i].ID == id {
			s.records = append(s.records[:i], s.records[i+1:]...)

			return true
		}
	}

	return false
}

// Snapshot copies the current queue.
func (s *Server) Snapshot() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Record, len(s.records))
	copy(out, s.records)

	return out
}

// QueueGets is how many authenticated GET /api/*/queue calls have been served.
func (s *Server) QueueGets() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.queueGets
}

func (s *Server) requireKey(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		got := request.Header.Get("X-Api-Key")
		if got == "" {
			got = strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		}

		if s.Key != "" && got != s.Key {
			http.Error(writer, `{"message":"Unauthorized"}`, http.StatusUnauthorized)

			return
		}

		next(writer, request)
	}
}

func (s *Server) serveQueue(writer http.ResponseWriter, request *http.Request) {
	s.mu.Lock()
	s.queueGets++
	s.mu.Unlock()

	page, _ := strconv.Atoi(request.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(request.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 10
	}

	sortKey := request.URL.Query().Get("sortKey")
	if sortKey == "" {
		sortKey = "timeleft"
	}

	sortDir := request.URL.Query().Get("sortDirection")
	if sortDir == "" {
		sortDir = "ascending"
	}

	all := s.Snapshot()
	pageRecs := paginate(all, page, pageSize)

	writer.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(writer).Encode(Queue{
		Page:          page,
		PageSize:      pageSize,
		SortKey:       sortKey,
		SortDirection: sortDir,
		TotalRecords:  len(all),
		Records:       pageRecs,
	})
}

// paginate returns one page without overflowing (page-1)*pageSize.
func paginate(all []Record, page, pageSize int) []Record {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 10
	}

	if page > 1 && (page-1) > len(all)/pageSize {
		return []Record{}
	}

	start := (page - 1) * pageSize
	if start >= len(all) {
		return []Record{}
	}

	end := len(all)
	if pageSize <= len(all)-start {
		end = start + pageSize
	}

	return all[start:end]
}

func (s *Server) serveAdd(writer http.ResponseWriter, request *http.Request) {
	var rec Record
	if err := json.NewDecoder(request.Body).Decode(&rec); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)

		return
	}

	rec = s.Add(rec)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(rec)
}

func (s *Server) serveComplete(writer http.ResponseWriter, request *http.Request) {
	id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
	if err != nil || !s.Complete(id) {
		http.NotFound(writer, request)

		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

func (s *Server) serveDrop(writer http.ResponseWriter, request *http.Request) {
	id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
	if err != nil || !s.Drop(id) {
		http.NotFound(writer, request)

		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

func (s *Server) serveDebugQueue(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(s.Snapshot())
}
