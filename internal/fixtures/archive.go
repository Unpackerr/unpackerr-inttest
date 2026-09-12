package fixtures

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Require skips the test when any named tool is missing from PATH.
func Require(t *testing.T, names ...string) {
	t.Helper()

	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("missing %s in PATH", name)
		}
	}
}

// SevenZip returns 7z, 7zz, or 7za, or empty.
func SevenZip() string {
	for _, name := range []string{"7z", "7zz", "7za"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}

	return ""
}

// Require7z skips when no 7-Zip CLI is installed.
func Require7z(t *testing.T) string {
	t.Helper()

	bin := SevenZip()
	if bin == "" {
		t.Skip("missing 7z/7zz/7za in PATH")
	}

	return bin
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

func absDest(t *testing.T, dest string) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}

	abs, err := filepath.Abs(dest)
	if err != nil {
		t.Fatal(err)
	}

	return abs
}

// RAROptions control rar CLI flags.
type RAROptions struct {
	Password   string
	Volume     string // e.g. "8k"
	OldVolumes bool   // -vn → .rar/.r00 instead of .part1.rar
}

// RAR archives srcDir into dest (.rar) using store compression.
func RAR(t *testing.T, dest, srcDir string, opts RAROptions) {
	t.Helper()
	Require(t, "rar")

	dest = absDest(t, dest)
	args := []string{"a", "-ep1", "-m0", "-y", "-inul"}

	if opts.Password != "" {
		args = append(args, "-p"+opts.Password)
	}

	if opts.Volume != "" {
		if opts.OldVolumes {
			args = append(args, "-vn")
		}

		args = append(args, "-v"+opts.Volume)
	}

	args = append(args, dest, ".")
	run(t, srcDir, "rar", args...)
}

// ZIP archives srcDir into dest. A non-empty password is passed to zip -P for
// fixture creation only; Unpackerr cannot extract encrypted zip (RAR/7z only).
func ZIP(t *testing.T, dest, srcDir, password string) {
	t.Helper()
	Require(t, "zip")

	dest = absDest(t, dest)
	args := []string{"-r", "-0", "-q"}

	if password != "" {
		args = append(args, "-P", password)
	}

	args = append(args, dest, ".")
	run(t, srcDir, "zip", args...)
}

// SevenZipArchive archives srcDir into dest (.7z).
func SevenZipArchive(t *testing.T, dest, srcDir, password string) {
	t.Helper()

	bin := Require7z(t)
	dest = absDest(t, dest)
	args := []string{"a", "-mx=0", "-bd"}

	if password != "" {
		args = append(args, "-p"+password)
	}

	args = append(args, dest, ".")
	run(t, srcDir, bin, args...)
}

// NestedZIP writes an inner zip inside an outer zip (outer contains inner.zip + optional extras).
func NestedZIP(t *testing.T, dest, innerSrc, extraSrc string) {
	t.Helper()
	Require(t, "zip")

	tmp := t.TempDir()
	inner := filepath.Join(tmp, "inner.zip")
	ZIP(t, inner, innerSrc, "")

	outerSrc := filepath.Join(tmp, "outer")
	if err := os.MkdirAll(outerSrc, 0o755); err != nil {
		t.Fatal(err)
	}

	innerBytes, err := os.ReadFile(inner)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(outerSrc, "inner.zip"), innerBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	if extraSrc != "" {
		entries, err := os.ReadDir(extraSrc)
		if err != nil {
			t.Fatal(err)
		}

		for _, ent := range entries {
			if ent.IsDir() {
				continue
			}

			data, err := os.ReadFile(filepath.Join(extraSrc, ent.Name()))
			if err != nil {
				t.Fatal(err)
			}

			if err := os.WriteFile(filepath.Join(outerSrc, ent.Name()), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	ZIP(t, dest, outerSrc, "")
}

// RARinZIP puts a rar inside a zip.
func RARinZIP(t *testing.T, dest, srcDir string) {
	t.Helper()
	Require(t, "rar", "zip")

	tmp := t.TempDir()
	rarPath := filepath.Join(tmp, "inner.rar")
	RAR(t, rarPath, srcDir, RAROptions{})

	wrap := filepath.Join(tmp, "wrap")
	if err := os.MkdirAll(wrap, 0o755); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(rarPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(wrap, "inner.rar"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	ZIP(t, dest, wrap, "")
}

// DummyISO writes a tiny non-ISO so extract_isos=false has something to ignore.
func DummyISO(t *testing.T, dest string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}

	// Not a valid ISO; Unpackerr should leave it when extract_isos=false.
	err := os.WriteFile(dest, []byte("not an iso\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
}

// DirWithPayload is a helper that creates dir, writes a payload, and returns it.
func DirWithPayload(t *testing.T, parent, name string) string {
	t.Helper()

	dir := filepath.Join(parent, name)
	if err := MustPayload(dir); err != nil {
		t.Fatal(err)
	}

	return dir
}

// DownloadSet is a Starr outputPath folder plus archives created from a payload.
func DownloadSet(t *testing.T, parent, group string) (outputPath, payloadDir string) {
	t.Helper()

	name := SceneName(group)
	outputPath = filepath.Join(parent, name)
	payloadDir = DirWithPayload(t, parent, name+"-src")

	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		t.Fatal(err)
	}

	return outputPath, payloadDir
}

// ListMatching returns names in dir with the given suffix (case-insensitive).
func ListMatching(t *testing.T, dir, suffix string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var out []string

	low := strings.ToLower(suffix)
	for _, ent := range entries {
		if strings.HasSuffix(strings.ToLower(ent.Name()), low) {
			out = append(out, filepath.Join(dir, ent.Name()))
		}
	}

	if len(out) == 0 {
		t.Fatalf("no %s files in %s", suffix, dir)
	}

	return out
}

// MustExist fails if path is missing.
func MustExist(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("missing %s: %v", path, err)
	}
}

// WalkHasBase reports whether a file named base exists under root.
func WalkHasBase(root, base string) bool {
	found := false

	_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil //nolint:nilerr
		}

		if d.Name() == base {
			found = true
		}

		return nil
	})

	return found
}

// WalkReadBase returns the contents of the first file named base under root.
func WalkReadBase(root, base string) (string, bool, error) {
	var (
		found string
		ok    bool
		walk  error
	)

	walk = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if d.Name() != base {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		found = string(data)
		ok = true

		return filepath.SkipAll
	})

	return found, ok, walk
}
