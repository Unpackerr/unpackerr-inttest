// Package fixtures builds small scene-like archives at test time.
package fixtures

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
)

const (
	// Marker is the known text packed into every payload.
	Marker = "unpackerr-inttest-payload\n"
	// MarkerName is the text file Unpackerr should extract.
	MarkerName = "payload.txt"
	// DefaultSeed is the RNG seed for urandom bytes.
	DefaultSeed = 1
	// DefaultRandom is the random.bin size.
	DefaultRandom = 4096
)

// Payload writes marker, NFO, and seeded random bytes into dir.
func Payload(dir string, seed int64, randomBytes int) error {
	if randomBytes <= 0 {
		randomBytes = DefaultRandom
	}

	if seed == 0 {
		seed = DefaultSeed
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir payload: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, MarkerName), []byte(Marker), 0o644); err != nil {
		return fmt.Errorf("payload.txt: %w", err)
	}

	nfo := []byte("Some.Show.S01E01.1080p.WEB.x264-GROUP\nhttps://example.invalid/\n")
	if err := os.WriteFile(filepath.Join(dir, "release.nfo"), nfo, 0o644); err != nil {
		return fmt.Errorf("release.nfo: %w", err)
	}

	buf := make([]byte, randomBytes)
	rng := rand.New(rand.NewPCG(uint64(seed), 1)) //nolint:gosec

	for i := range buf {
		buf[i] = byte(rng.UintN(256))
	}

	if err := os.WriteFile(filepath.Join(dir, "random.bin"), buf, 0o644); err != nil {
		return fmt.Errorf("random.bin: %w", err)
	}

	return nil
}

// SceneName returns a small scene-style release folder name.
func SceneName(group string) string {
	if group == "" {
		group = "INTTEST"
	}

	return "Some.Show.S01E01.1080p.WEB.x264-" + group
}

// MustPayload writes a payload or fails the test helper callback.
func MustPayload(dir string) error {
	return Payload(dir, DefaultSeed, DefaultRandom)
}
