package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CreateRunDir creates a unique run directory below base/runs and returns its
// ID and absolute path. base defaults to .pysh.
func CreateRunDir(base string) (id, dir string, err error) {
	if base == "" {
		base = ".pysh"
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", "", fmt.Errorf("resolve work directory: %w", err)
	}
	runsDir := filepath.Join(absBase, "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create runs directory: %w", err)
	}

	stamp := time.Now().UTC().Format("20060102-150405")
	for attempt := 0; attempt < 10; attempt++ {
		var suffix [3]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return "", "", fmt.Errorf("generate run id: %w", err)
		}
		id = stamp + "-" + hex.EncodeToString(suffix[:])
		dir = filepath.Join(runsDir, id)
		if err := os.Mkdir(dir, 0o755); err == nil {
			if err := os.Mkdir(filepath.Join(dir, "tasks"), 0o755); err != nil {
				return "", "", fmt.Errorf("create task log directory: %w", err)
			}
			return id, dir, nil
		} else if !os.IsExist(err) {
			return "", "", fmt.Errorf("create run directory: %w", err)
		}
	}
	return "", "", fmt.Errorf("could not create a unique run directory")
}
