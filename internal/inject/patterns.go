package inject

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Unpackerr/unpackerr-inttest/internal/fixtures"
)

const pathPassword = "hunter2"

type patternFunc func(outputPath, src string, bytes int, compress bool) error

var patterns = map[string]patternFunc{
	"rar":       stageRAR,
	"zip":       stageZIP,
	"7z":        stage7z,
	"recursive": stageRecursive,
	"nested":    stageRecursive,
	"nests":     stageNests,
	"subs":      stageSubs,
	"depth":     stageDepth,
	"volumes":   stageVolumes,
	"r00":       stageR00,
	"password":  stagePassword,
	"empty":     stageEmpty,
	"rarzip":    stageRARinZIP,
	"syncthing": stageSyncthing,
}

// PatternNames is the -b enum (aliases omitted).
func PatternNames() []string {
	return []string{
		"rar", "zip", "7z", "recursive", "nests", "subs", "depth",
		"volumes", "r00", "password", "empty", "rarzip", "syncthing",
	}
}

// Stage writes a builtin layout under destRoot and returns title + outputPath.
func Stage(pattern, destRoot, app string, bytes int, compress bool) (title, outputPath string, err error) {
	fn, ok := patterns[strings.ToLower(pattern)]
	if !ok {
		return "", "", fmt.Errorf("unknown pattern %q", pattern)
	}

	if bytes < 1 {
		bytes = fixtures.DefaultRandom
	}

	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return "", "", fmt.Errorf("mkdir -d: %w", err)
	}

	destRoot, err = filepath.Abs(destRoot)
	if err != nil {
		return "", "", err
	}

	stamp := time.Now().Format("150405") + strconv.Itoa(int(time.Now().UnixNano()%1000))
	group := strings.ToUpper(app) + "-" + strings.ToUpper(strings.ReplaceAll(pattern, " ", "")) + "-" + stamp
	title = fixtures.SceneName(group)

	src, err := os.MkdirTemp("", "unpackerr-inject-src-")
	if err != nil {
		return "", "", fmt.Errorf("payload temp: %w", err)
	}
	defer os.RemoveAll(src)

	if err := fixtures.Payload(src, time.Now().UnixNano(), bytes); err != nil {
		return "", "", err
	}

	outputPath = filepath.Join(destRoot, title)
	if strings.ToLower(pattern) == "password" {
		outputPath = filepath.Join(destRoot, "Show.{{"+pathPassword+"}}."+group)
	}

	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		return "", "", fmt.Errorf("mkdir outputPath: %w", err)
	}

	if err := fn(outputPath, src, bytes, compress); err != nil {
		return "", "", err
	}

	return title, outputPath, nil
}

func stageRAR(outputPath, src string, _ int, compress bool) error {
	return fixtures.WriteRAR(filepath.Join(outputPath, "show.rar"), src, fixtures.RAROptions{Compress: compress})
}

func stageZIP(outputPath, src string, _ int, compress bool) error {
	return fixtures.WriteZIP(filepath.Join(outputPath, "show.zip"), src, "", compress)
}

func stage7z(outputPath, src string, _ int, compress bool) error {
	return fixtures.WriteSevenZip(filepath.Join(outputPath, "show.7z"), src, "", compress)
}

func stageRecursive(outputPath, src string, _ int, compress bool) error {
	if err := os.WriteFile(filepath.Join(src, "nested-only.txt"), []byte("inner payload\n"), 0o644); err != nil {
		return err
	}

	return fixtures.WriteNestedZIP(filepath.Join(outputPath, "show.zip"), src, "", compress)
}

func stageNests(outputPath, src string, _ int, compress bool) error {
	wrap, err := os.MkdirTemp("", "unpackerr-nests-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(wrap)

	for _, name := range []string{"one", "two", "three"} {
		if err := fixtures.WriteZIP(filepath.Join(wrap, name+".zip"), src, "", compress); err != nil {
			return err
		}
	}

	return fixtures.WriteZIP(filepath.Join(outputPath, "show.zip"), wrap, "", compress)
}

func stageSubs(outputPath, src string, _ int, compress bool) error {
	season := filepath.Join(outputPath, "Season.01")
	if err := os.MkdirAll(season, 0o755); err != nil {
		return err
	}

	return fixtures.WriteRAR(filepath.Join(season, "show.rar"), src, fixtures.RAROptions{Compress: compress})
}

func stageDepth(outputPath, _ string, bytes int, compress bool) error {
	deepSrc, err := os.MkdirTemp("", "unpackerr-deep-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(deepSrc)

	nearSrc, err := os.MkdirTemp("", "unpackerr-near-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(nearSrc)

	if err := fixtures.Payload(deepSrc, 2, bytes); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(deepSrc, "deep.txt"), []byte("buried\n"), 0o644); err != nil {
		return err
	}

	if err := fixtures.Payload(nearSrc, 3, bytes); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(nearSrc, "near.txt"), []byte("ok\n"), 0o644); err != nil {
		return err
	}

	deepDir := filepath.Join(outputPath, "a", "b", "c")
	nearDir := filepath.Join(outputPath, "shallow")

	if err := os.MkdirAll(deepDir, 0o755); err != nil {
		return err
	}

	if err := os.MkdirAll(nearDir, 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(outputPath, "keep.txt"), []byte("surface\n"), 0o644); err != nil {
		return err
	}

	if err := fixtures.WriteZIP(filepath.Join(deepDir, "inner.zip"), deepSrc, "", compress); err != nil {
		return err
	}

	if err := fixtures.WriteZIP(filepath.Join(nearDir, "sibling.zip"), nearSrc, "", compress); err != nil {
		return err
	}

	return nil
}

func stageVolumes(outputPath, src string, bytes int, compress bool) error {
	return fixtures.WriteRAR(filepath.Join(outputPath, "show.rar"), src, fixtures.RAROptions{
		Volume:   volumeFlag(bytes),
		Compress: compress,
	})
}

func stageR00(outputPath, src string, _ int, compress bool) error {
	rarPath := filepath.Join(outputPath, "show.rar")
	if err := fixtures.WriteRAR(rarPath, src, fixtures.RAROptions{Compress: compress}); err != nil {
		return err
	}

	return fixtures.CopyFile(rarPath, filepath.Join(outputPath, "show.r00"))
}

func stagePassword(outputPath, src string, _ int, compress bool) error {
	return fixtures.WriteRAR(filepath.Join(outputPath, "show.rar"), src, fixtures.RAROptions{
		Password: pathPassword,
		Compress: compress,
	})
}

func stageEmpty(outputPath, src string, _ int, _ bool) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}

		if err := fixtures.CopyFile(filepath.Join(src, ent.Name()), filepath.Join(outputPath, ent.Name())); err != nil {
			return err
		}
	}

	return nil
}

func stageRARinZIP(outputPath, src string, _ int, compress bool) error {
	return fixtures.WriteRARinZIP(filepath.Join(outputPath, "show.zip"), src, compress)
}

func stageSyncthing(outputPath, src string, _ int, compress bool) error {
	if err := fixtures.WriteRAR(filepath.Join(outputPath, "show.rar"), src, fixtures.RAROptions{
		Compress: compress,
	}); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(outputPath, "file.tmp"), []byte("syncing"), 0o644)
}

func volumeFlag(bytes int) string {
	chunk := max(bytes/3, 2048)

	return strconv.Itoa(chunk/1024) + "k"
}
