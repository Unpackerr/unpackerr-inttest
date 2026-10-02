package fixtures_test

import (
	"testing"

	"github.com/Unpackerr/unpackerr-inttest/internal/fixtures"
)

func TestIssue796RatiosStraddleCap(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ratios := fixtures.WriteIssue796(t, dir)

	t.Logf("true=%.3f inflated=%.3f nested=%.3f", ratios.True, ratios.Inflated, ratios.Nested)
}
