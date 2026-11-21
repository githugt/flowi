// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GitLineMerger is a LineMerger that asks git (`git merge-file`), which is there whenever git runs a merge driver.
// Conflicts come back as the usual <<<<<<< ======= >>>>>>> markers.
func GitLineMerger(ctx context.Context) LineMerger {
	return func(base, ours, theirs []byte) ([]byte, bool, error) {
		dir, err := os.MkdirTemp("", "flowi-merge-")
		if err != nil {
			return nil, false, err
		}
		defer os.RemoveAll(dir)
		files := map[string][]byte{"ours": ours, "base": base, "theirs": theirs}
		for name, data := range files {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
				return nil, false, err
			}
		}
		var out, stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, "git", "merge-file", "-p", "-L", "ours", "-L", "base", "-L", "theirs", //nolint:gosec // a fixed command; the files are in a temporary folder made here
			filepath.Join(dir, "ours"), filepath.Join(dir, "base"), filepath.Join(dir, "theirs"))
		cmd.Stdout, cmd.Stderr = &out, &stderr
		err = cmd.Run()
		var exit *exec.ExitError
		switch {
		case err == nil:
			return out.Bytes(), true, nil
		case errors.As(err, &exit) && exit.ExitCode() > 0 && exit.ExitCode() < 128:
			return out.Bytes(), false, nil // that many conflicts
		}
		return nil, false, fmt.Errorf("git merge-file: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
}
