package harness_test

import (
	"testing"

	"github.com/Unpackerr/unpackerr-inttest/internal/harness"
)

func TestAPIKeyLengths(t *testing.T) {
	t.Parallel()

	if n := len(harness.DefaultAPIKey); n < 60 || n > 150 {
		t.Fatalf("DefaultAPIKey length %d", n)
	}

	if n := len(harness.StarrAPIKey); n < 32 {
		t.Fatalf("StarrAPIKey length %d", n)
	}
}
