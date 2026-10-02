package fixtures

import (
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

const (
	// SceneFeatureName is the file inside the top-level movie archive.
	SceneFeatureName = "feature.bin"
	// SceneFeature is that file's contents.
	SceneFeature = "unpackerr-inttest-feature\n"
	// SceneSubName is the file inside the nested subs rar.
	SceneSubName = "sub.bin"
	// SceneSubBytes is the uncompressed subtitle payload.
	// Issue 796's .sub was 8,957,952 bytes inside a 1,845,399 byte rar (4.85x).
	// This is the same ratio at a size the test can build quickly.
	SceneSubBytes     = 2_000_000
	sceneNestedTarget = 4.85
	sceneNestedMin    = 4.70
	sceneNestedMax    = 4.98
)

// SceneRatios is the issue 796 accounting for one built subs archive.
// True is (.idx + .sub) / subs.rar. Inflated also counts the nested rar,
// which is what failed the old MaxRatio of 5.
type SceneRatios struct {
	True     float64
	Inflated float64
	Nested   float64
}

// WriteIssue796 builds a scene download: a movie rar plus Subs/show.subs.rar.
// The subs rar stores a small idx and a nested rar. The nested rar expands
// about 4.85x. Final output stays under 5x the subs rar; counting the nested
// rar as well goes over 5. https://github.com/Unpackerr/unpackerr/issues/796
func WriteIssue796(t *testing.T, download string) SceneRatios {
	t.Helper()
	Require(t, "rar")

	feature := t.TempDir()
	err := os.WriteFile(filepath.Join(feature, SceneFeatureName), []byte(SceneFeature), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	if err = WriteRAR(filepath.Join(download, "movie.rar"), feature, RAROptions{}); err != nil {
		t.Fatal(err)
	}

	subsDir := filepath.Join(download, "Subs")
	if err = os.MkdirAll(subsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ratios, err := writeSceneSubs(filepath.Join(subsDir, "show.subs.rar"))
	if err != nil {
		t.Fatal(err)
	}

	if ratios.True >= 5 || ratios.Inflated <= 5 || ratios.Nested < sceneNestedMin || ratios.Nested > sceneNestedMax {
		t.Fatalf("scene ratios true=%.3f inflated=%.3f nested=%.3f, want true<5 inflated>5 nested in [%.2f,%.2f]",
			ratios.True, ratios.Inflated, ratios.Nested, sceneNestedMin, sceneNestedMax)
	}

	return ratios
}

func writeSceneSubs(dest string) (SceneRatios, error) {
	lo, hi := 200_000, 500_000
	bestN := (lo + hi) / 2
	bestDist := math.MaxFloat64

	for lo <= hi {
		mid := (lo + hi) / 2

		ratios, err := packSceneSubs(dest, mid)
		if err != nil {
			return SceneRatios{}, err
		}

		dist := math.Abs(ratios.Nested - sceneNestedTarget)
		if dist < bestDist {
			bestDist = dist
			bestN = mid
		}

		switch {
		case ratios.Nested > sceneNestedTarget:
			lo = mid + 1
		case ratios.Nested < sceneNestedTarget:
			hi = mid - 1
		default:
			lo = hi + 1
		}
	}

	return packSceneSubs(dest, bestN)
}

func packSceneSubs(dest string, randomN int) (SceneRatios, error) {
	work, err := os.MkdirTemp("", "unpackerr-scene-subs-")
	if err != nil {
		return SceneRatios{}, fmt.Errorf("scene temp: %w", err)
	}
	defer os.RemoveAll(work)

	inner := filepath.Join(work, "inner")
	if err = os.MkdirAll(inner, 0o755); err != nil {
		return SceneRatios{}, fmt.Errorf("scene inner: %w", err)
	}

	if err = os.WriteFile(filepath.Join(inner, SceneSubName), sceneSubPayload(randomN), 0o644); err != nil {
		return SceneRatios{}, fmt.Errorf("scene sub: %w", err)
	}

	nested := filepath.Join(work, "nested.rar")
	if err = archiveRAR(nested, inner, "-m5", SceneSubName); err != nil {
		return SceneRatios{}, err
	}

	outer := filepath.Join(work, "outer")
	if err = os.MkdirAll(outer, 0o755); err != nil {
		return SceneRatios{}, fmt.Errorf("scene outer: %w", err)
	}

	idx := bytesRepeat("VobSub index stand-in\n", 20)
	if err = os.WriteFile(filepath.Join(outer, "movie.idx"), idx, 0o644); err != nil {
		return SceneRatios{}, fmt.Errorf("scene idx: %w", err)
	}

	if err = CopyFile(nested, filepath.Join(outer, "nested.rar")); err != nil {
		return SceneRatios{}, err
	}

	if err = archiveRAR(dest, outer, "-m0", "movie.idx", "nested.rar"); err != nil {
		return SceneRatios{}, err
	}

	return sceneRatio(dest, filepath.Join(outer, "movie.idx"), filepath.Join(outer, "nested.rar"))
}

func sceneSubPayload(randomN int) []byte {
	buf := make([]byte, SceneSubBytes)
	if randomN <= 0 {
		return buf
	}

	if randomN > len(buf) {
		randomN = len(buf)
	}

	rng := rand.New(rand.NewPCG(1, 1)) //nolint:gosec // fixture bytes, not a secret

	for i := range randomN {
		buf[i] = byte(rng.UintN(256))
	}

	return buf
}

func bytesRepeat(s string, n int) []byte {
	out := make([]byte, 0, len(s)*n)
	for range n {
		out = append(out, s...)
	}

	return out
}

func archiveRAR(dest, dir, level string, names ...string) error {
	dest, err := AbsDest(dest)
	if err != nil {
		return err
	}

	args := make([]string, 0, 6+len(names))
	args = append(args, "a", "-ep", level, "-y", "-inul", dest)
	args = append(args, names...)

	return Run(dir, "rar", args...)
}

func sceneRatio(subs, idx, nested string) (SceneRatios, error) {
	parent, err := os.Stat(subs)
	if err != nil {
		return SceneRatios{}, fmt.Errorf("stat subs: %w", err)
	}

	idxInfo, err := os.Stat(idx)
	if err != nil {
		return SceneRatios{}, fmt.Errorf("stat idx: %w", err)
	}

	nestedInfo, err := os.Stat(nested)
	if err != nil {
		return SceneRatios{}, fmt.Errorf("stat nested: %w", err)
	}

	p := float64(parent.Size())
	s := float64(idxInfo.Size())
	i := float64(nestedInfo.Size())
	u := float64(SceneSubBytes)

	return SceneRatios{
		True:     (s + u) / p,
		Inflated: (s + i + u) / p,
		Nested:   u / i,
	}, nil
}
