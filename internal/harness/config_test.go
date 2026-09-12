package harness

import (
	"strings"
	"testing"
	"time"
)

func TestRenderConfig(t *testing.T) {
	t.Parallel()

	h := &H{
		Addr:    "127.0.0.1:5656",
		APIKey:  DefaultAPIKey,
		LogFile: "/tmp/unpackerr.log",
	}
	body := renderConfig(h, defaultOptions(Options{
		Passwords: []string{Password},
		Starr: []Starr{{
			App:         "sonarr",
			URL:         "http://127.0.0.1:8989",
			APIKey:      StarrAPIKey,
			Protocols:   "torrent",
			DeleteDelay: time.Second,
		}},
		Folders: []Folder{{
			Path:         "/watch",
			ExcludePaths: []string{"skip"},
			MaxNested:    2,
		}},
	}))

	for _, want := range []string{
		`ui_password = "noauth"`,
		`listen_addr = "127.0.0.1:5656"`,
		`[[webserver.api_keys]]`,
		`[[sonarr]]`,
		`api_key = "` + StarrAPIKey + `"`,
		`protocols = "torrent"`,
		`[[folder]]`,
		`path = "/watch"`,
		`exclude_paths = ["skip"]`,
		`max_nested = 2`,
		`passwords = ["` + Password + `"]`,
		`[folders]`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}
