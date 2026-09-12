package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// EnsureBinary returns UNPACKERR_BIN, a sibling ../unpackerr/unpackerr (built if needed),
// or an error that tests should t.Skip.
func EnsureBinary(explicit string) (string, error) {
	if explicit != "" {
		return existingFile(explicit)
	}

	if env := os.Getenv("UNPACKERR_BIN"); env != "" {
		return existingFile(env)
	}

	root, err := moduleRoot()
	if err != nil {
		return "", err
	}

	sibling := filepath.Join(filepath.Dir(root), "unpackerr")
	bin := filepath.Join(sibling, "unpackerr")

	if path, err := existingFile(bin); err == nil {
		return path, nil
	}

	if _, err := os.Stat(filepath.Join(sibling, "go.mod")); err != nil {
		return "", fmt.Errorf("set UNPACKERR_BIN or clone Unpackerr/unpackerr next to this repo (%s)", sibling)
	}

	cmd := exec.Command("go", "build", "-o", "unpackerr", ".")
	cmd.Dir = sibling
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("build %s: %w\n%s", sibling, err, out)
	}

	return existingFile(bin)
}

func existingFile(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	stat, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("unpackerr binary %s: %w", abs, err)
	}

	if stat.IsDir() {
		return "", fmt.Errorf("unpackerr binary %s is a directory", abs)
	}

	return abs, nil
}

func moduleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}

		dir = parent
	}
}
