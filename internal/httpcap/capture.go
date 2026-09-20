// Package httpcap is an httptest server that records JSON webhook bodies.
package httpcap

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Server records POST bodies from Unpackerr webhooks.
type Server struct {
	URL  string
	mu   sync.Mutex
	raw  [][]byte
	http *httptest.Server
}

// Start listens on loopback and closes with the test.
func Start(t *testing.T) *Server {
	t.Helper()

	server := &Server{}
	server.http = httptest.NewServer(http.HandlerFunc(server.handle))
	t.Cleanup(server.http.Close)
	server.URL = server.http.URL

	return server
}

func (s *Server) handle(writer http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.raw = append(s.raw, body)
	s.mu.Unlock()
	writer.WriteHeader(http.StatusNoContent)
}

// WaitEvent waits for a payload whose unpackerr_eventtype matches event.
func (s *Server) WaitEvent(t *testing.T, event string, timeout time.Duration) map[string]any {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, payload := range s.payloads() {
			got, _ := payload["unpackerr_eventtype"].(string)
			if got == event {
				return payload
			}
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("timeout waiting for webhook event %q; last=%v", event, s.payloads())

	return nil
}

func (s *Server) payloads() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]map[string]any, 0, len(s.raw))
	for _, raw := range s.raw {
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			continue
		}

		out = append(out, payload)
	}

	return out
}
