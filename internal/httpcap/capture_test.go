package httpcap_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Unpackerr/unpackerr-inttest/internal/httpcap"
)

func TestWaitEvent(t *testing.T) {
	t.Parallel()

	server := httpcap.Start(t)
	body := `{"unpackerr_eventtype":"extracted","eventTitle":"ok"}`

	res, err := http.Post(server.URL, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	_ = res.Body.Close()

	got := server.WaitEvent(t, "extracted", time.Second)
	if got["eventTitle"] != "ok" {
		t.Fatalf("payload %+v", got)
	}
}
