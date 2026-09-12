package inject

import (
	"fmt"
	"strconv"
	"strings"
)

const maxPayload = 2 << 30 // 2 GiB

// ParseSize accepts 4096, 4k, 32m, 1g (binary units).
func ParseSize(raw string) (int, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		s = defaultSize
	}

	mult := 1
	switch {
	case strings.HasSuffix(s, "kb"):
		mult = 1024
		s = strings.TrimSuffix(s, "kb")
	case strings.HasSuffix(s, "mb"):
		mult = 1024 * 1024
		s = strings.TrimSuffix(s, "mb")
	case strings.HasSuffix(s, "gb"):
		mult = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "gb")
	case strings.HasSuffix(s, "k"):
		mult = 1024
		s = strings.TrimSuffix(s, "k")
	case strings.HasSuffix(s, "m"):
		mult = 1024 * 1024
		s = strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "g"):
		mult = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "g")
	}

	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid -size %q", raw)
	}

	bytes := n * mult
	if bytes > maxPayload {
		return 0, fmt.Errorf("-size %q is larger than 2GiB", raw)
	}

	return bytes, nil
}
