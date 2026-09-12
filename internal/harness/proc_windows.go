//go:build windows

package harness

import (
	"os/exec"
	"time"
)

func prepareCmd(_ *exec.Cmd) {}

func stopCmd(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	_ = cmd.Process.Kill()

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}
