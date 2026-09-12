// Package write slowly writes files so folder start_delay can observe in-progress copies.
package write

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// File writes data to path in chunks, sleeping between writes (not after the last).
func File(path string, data []byte, chunk int, pause time.Duration) error {
	if chunk <= 0 {
		chunk = 32
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}

	defer file.Close()

	for len(data) > 0 {
		n := min(chunk, len(data))

		if _, err := file.Write(data[:n]); err != nil {
			return fmt.Errorf("write: %w", err)
		}

		if err := file.Sync(); err != nil {
			return fmt.Errorf("sync: %w", err)
		}

		data = data[n:]
		if len(data) > 0 && pause > 0 {
			time.Sleep(pause)
		}
	}

	return nil
}
