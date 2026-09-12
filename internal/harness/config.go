package harness

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func renderConfig(h *H, opts Options) string {
	var b strings.Builder

	fmt.Fprintf(&b, "debug = %v\n", opts.Debug)
	fmt.Fprintf(&b, "quiet = false\n")
	fmt.Fprintf(&b, "interval = %q\n", dur(opts.Interval))
	fmt.Fprintf(&b, "start_delay = %q\n", dur(opts.StartDelay))
	fmt.Fprintf(&b, "delete_delay = %q\n", dur(opts.DeleteDelay))
	fmt.Fprintf(&b, "retry_delay = %q\n", dur(opts.RetryDelay))
	fmt.Fprintf(&b, "progress = %q\n", dur(opts.Progress))
	fmt.Fprintf(&b, "log_queues = %q\n", dur(opts.LogQueues))
	fmt.Fprintf(&b, "keep_history = %d\n", opts.KeepHistory)
	fmt.Fprintf(&b, "parallel = 1\n")
	fmt.Fprintf(&b, "log_file = %s\n", quote(h.LogFile))
	fmt.Fprintf(&b, "passwords = [%s]\n", joinQuotes(opts.Passwords))
	fmt.Fprintf(&b, "\n[webserver]\n")
	fmt.Fprintf(&b, "listen_addr = %s\n", quote(h.Addr))
	fmt.Fprintf(&b, "ui_password = \"noauth\"\n")
	fmt.Fprintf(&b, "\n[[webserver.api_keys]]\n")
	fmt.Fprintf(&b, "name = \"admin\"\n")
	fmt.Fprintf(&b, "key = %s\n", quote(h.APIKey))
	fmt.Fprintf(&b, "roles = [\"admin\"]\n")

	if len(opts.Folders) > 0 {
		fmt.Fprintf(&b, "\n[folders]\n")
		fmt.Fprintf(&b, "interval = %q\n", dur(opts.FolderPoll))
	}

	for _, starr := range opts.Starr {
		section := starr.App
		if section == "" {
			section = "sonarr"
		}

		fmt.Fprintf(&b, "\n[[%s]]\n", section)
		fmt.Fprintf(&b, "url = %s\n", quote(starr.URL))

		key := starr.APIKey
		if key == "" {
			key = StarrAPIKey
		}

		fmt.Fprintf(&b, "api_key = %s\n", quote(key))

		if starr.Protocols != "" {
			fmt.Fprintf(&b, "protocols = %s\n", quote(starr.Protocols))
		}

		if starr.Path != "" {
			fmt.Fprintf(&b, "path = %s\n", quote(starr.Path))
		}

		timeout := starr.Timeout
		if timeout == 0 {
			timeout = 5 * time.Second
		}

		fmt.Fprintf(&b, "timeout = %q\n", dur(timeout))
		fmt.Fprintf(&b, "syncthing = %v\n", starr.Syncthing)
		fmt.Fprintf(&b, "delete_orig = %v\n", starr.DeleteOrig)

		if starr.DeleteDelay > 0 {
			fmt.Fprintf(&b, "delete_delay = %q\n", dur(starr.DeleteDelay))
		}
	}

	for _, folder := range opts.Folders {
		fmt.Fprintf(&b, "\n[[folder]]\n")
		fmt.Fprintf(&b, "path = %s\n", quote(folder.Path))

		if folder.ExtractPath != "" {
			fmt.Fprintf(&b, "extract_path = %s\n", quote(folder.ExtractPath))
		}

		if len(folder.ExcludePaths) > 0 {
			fmt.Fprintf(&b, "exclude_paths = [%s]\n", joinQuotes(folder.ExcludePaths))
		}

		fmt.Fprintf(&b, "disable_recursion = %v\n", folder.DisableRecursion)
		fmt.Fprintf(&b, "extract_isos = %v\n", folder.ExtractISOs)
		fmt.Fprintf(&b, "move_back = %v\n", folder.MoveBack)
		fmt.Fprintf(&b, "delete_original = %v\n", folder.DeleteOrig)
		fmt.Fprintf(&b, "delete_files = %v\n", folder.DeleteFiles)
		fmt.Fprintf(&b, "delete_after = %q\n", dur(folder.DeleteAfter))

		if folder.MaxNested != 0 {
			fmt.Fprintf(&b, "max_nested = %d\n", folder.MaxNested)
		}

		if folder.ExtrasMaxDepth != 0 {
			fmt.Fprintf(&b, "extras_max_depth = %d\n", folder.ExtrasMaxDepth)
		}
	}

	return b.String()
}

func dur(d time.Duration) string {
	return d.String()
}

func quote(s string) string {
	return strconv.Quote(s)
}

func joinQuotes(values []string) string {
	if len(values) == 0 {
		return ""
	}

	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.Quote(v)
	}

	return strings.Join(parts, ", ")
}
