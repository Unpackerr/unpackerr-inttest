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
			Key:            "watch",
			Path:           "/watch",
			Interval:       time.Second,
			ExcludePaths:   []string{"skip"},
			WaitExtensions: []string{".part"},
			SkipEmpty:      true,
			MaxNested:      2,
		}},
		Webhooks: []Webhook{{
			Key:    "capture",
			URL:    "http://127.0.0.1:9/hook",
			Events: []int{4, 5},
		}},
		HookIDs:    map[string]string{"url": "https://unpackerr.example"},
		HookTitles: map[string]string{"extracted": "Custom extracted"},
	}))

	for _, want := range []string{
		`ui_password = "noauth"`,
		`listen_addr = "127.0.0.1:5656"`,
		`[[webserver.api_keys]]`,
		`[sonarr.0]`,
		`api_key = "` + StarrAPIKey + `"`,
		`protocols = "torrent"`,
		`[folder.watch]`,
		`path = "/watch"`,
		`interval = "1s"`,
		`exclude_paths = ["skip"]`,
		`wait_extensions = [".part"]`,
		`skip_empty = true`,
		`max_nested = 2`,
		`passwords = ["` + Password + `"]`,
		`[hooks.custom_ids]`,
		`url = "https://unpackerr.example"`,
		`[hooks.titles]`,
		`extracted = "Custom extracted"`,
		`[webhook.capture]`,
		`events = [4, 5]`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}

	if strings.Contains(body, "[folders]") {
		t.Fatalf("global [folders] interval must not be written:\n%s", body)
	}

	if strings.Contains(body, "[[folder]]") || strings.Contains(body, "[[sonarr]]") {
		t.Fatalf("legacy array tables still present:\n%s", body)
	}
}
