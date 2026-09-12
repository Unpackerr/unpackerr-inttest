package fixtures

import (
	"errors"
	"fmt"
	"io"
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

// Run executes name in dir and returns a combined stdout/stderr error.
func Run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, out)
	}

	return nil
}

// AbsDest creates dest's parent and returns an absolute path.
func AbsDest(dest string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("mkdir dest: %w", err)
	}

	abs, err := filepath.Abs(dest)
	if err != nil {
		return "", fmt.Errorf("abs dest: %w", err)
	}

	return abs, nil
}

// CopyFile writes dest from src bytes.
func CopyFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkdir copy: %w", err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dest, err)
	}

	return nil
}

// RAROptions control rar CLI flags.
type RAROptions struct {
	Password   string
	Volume     string // e.g. "8k"
	OldVolumes bool   // -vn → .rar/.r00 instead of .part1.rar
	Compress   bool   // -m3 instead of store (-m0)
}

// WriteRAR archives srcDir into dest (.rar).
func WriteRAR(dest, srcDir string, opts RAROptions) error {
	if _, err := exec.LookPath("rar"); err != nil {
		return errors.New("rar not in PATH")
	}

	dest, err := AbsDest(dest)
	if err != nil {
		return err
	}

	level := "-m0"
	if opts.Compress {
		level = "-m3"
	}

	args := []string{"a", "-ep1", level, "-y", "-inul"}

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

	return Run(srcDir, "rar", args...)
}

// WriteZIP archives srcDir into dest. A non-empty password is passed to zip -P
// for fixture creation only; Unpackerr cannot extract encrypted zip (RAR/7z only).
func WriteZIP(dest, srcDir, password string, compress bool) error {
	if _, err := exec.LookPath("zip"); err != nil {
		return errors.New("zip not in PATH")
	}

	dest, err := AbsDest(dest)
	if err != nil {
		return err
	}

	args := []string{"-r", "-q"}
	if !compress {
		args = append(args, "-0")
	}

	if password != "" {
		args = append(args, "-P", password)
	}

	args = append(args, dest, ".")

	return Run(srcDir, "zip", args...)
}

// WriteSevenZip archives srcDir into dest (.7z).
func WriteSevenZip(dest, srcDir, password string, compress bool) error {
	bin := SevenZip()
	if bin == "" {
		return errors.New("7z/7zz/7za not in PATH")
	}

	dest, err := AbsDest(dest)
	if err != nil {
		return err
	}

	level := "-mx=0"
	if compress {
		level = "-mx=3"
	}

	args := []string{"a", level, "-bd"}

	if password != "" {
		args = append(args, "-p"+password)
	}

	args = append(args, dest, ".")

	return Run(srcDir, bin, args...)
}

// WriteNestedZIP writes an inner zip inside an outer zip (outer contains inner.zip + optional extras).
func WriteNestedZIP(dest, innerSrc, extraSrc string, compress bool) error {
	tmp, err := os.MkdirTemp("", "unpackerr-nestedzip-")
	if err != nil {
		return fmt.Errorf("temp nested zip: %w", err)
	}
	defer os.RemoveAll(tmp)

	inner := filepath.Join(tmp, "inner.zip")
	if err := WriteZIP(inner, innerSrc, "", compress); err != nil {
		return err
	}

	outerSrc := filepath.Join(tmp, "outer")
	if err := os.MkdirAll(outerSrc, 0o755); err != nil {
		return fmt.Errorf("mkdir outer: %w", err)
	}

	if err := CopyFile(inner, filepath.Join(outerSrc, "inner.zip")); err != nil {
		return err
	}

	if extraSrc != "" {
		entries, err := os.ReadDir(extraSrc)
		if err != nil {
			return fmt.Errorf("read extra: %w", err)
		}

		for _, ent := range entries {
			if ent.IsDir() {
				continue
			}

			if err := CopyFile(filepath.Join(extraSrc, ent.Name()), filepath.Join(outerSrc, ent.Name())); err != nil {
				return err
			}
		}
	}

	return WriteZIP(dest, outerSrc, "", compress)
}

// WriteRARinZIP puts a rar inside a zip.
func WriteRARinZIP(dest, srcDir string, compress bool) error {
	tmp, err := os.MkdirTemp("", "unpackerr-rarzip-")
	if err != nil {
		return fmt.Errorf("temp rarzip: %w", err)
	}
	defer os.RemoveAll(tmp)

	rarPath := filepath.Join(tmp, "inner.rar")
	if err := WriteRAR(rarPath, srcDir, RAROptions{Compress: compress}); err != nil {
		return err
	}

	wrap := filepath.Join(tmp, "wrap")
	if err := os.MkdirAll(wrap, 0o755); err != nil {
		return fmt.Errorf("mkdir wrap: %w", err)
	}

	if err := CopyFile(rarPath, filepath.Join(wrap, "inner.rar")); err != nil {
		return err
	}

	return WriteZIP(dest, wrap, "", compress)
}

// RAR archives srcDir into dest (.rar) using store compression.
func RAR(t *testing.T, dest, srcDir string, opts RAROptions) {
	t.Helper()
	Require(t, "rar")

	if err := WriteRAR(dest, srcDir, opts); err != nil {
		t.Fatal(err)
	}
}

// ZIP archives srcDir into dest. A non-empty password is passed to zip -P for
// fixture creation only; Unpackerr cannot extract encrypted zip (RAR/7z only).
func ZIP(t *testing.T, dest, srcDir, password string) {
	t.Helper()
	Require(t, "zip")

	if err := WriteZIP(dest, srcDir, password, false); err != nil {
		t.Fatal(err)
	}
}

// SevenZipArchive archives srcDir into dest (.7z).
func SevenZipArchive(t *testing.T, dest, srcDir, password string) {
	t.Helper()
	Require7z(t)

	if err := WriteSevenZip(dest, srcDir, password, false); err != nil {
		t.Fatal(err)
	}
}

// NestedZIP writes an inner zip inside an outer zip (outer contains inner.zip + optional extras).
func NestedZIP(t *testing.T, dest, innerSrc, extraSrc string) {
	t.Helper()
	Require(t, "zip")

	if err := WriteNestedZIP(dest, innerSrc, extraSrc, false); err != nil {
		t.Fatal(err)
	}
}

// RARinZIP puts a rar inside a zip.
func RARinZIP(t *testing.T, dest, srcDir string) {
	t.Helper()
	Require(t, "rar", "zip")

	if err := WriteRARinZIP(dest, srcDir, false); err != nil {
		t.Fatal(err)
	}
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
