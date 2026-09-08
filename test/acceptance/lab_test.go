//go:build acceptance

package acceptance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type run struct {
	command        string
	code           int
	stdout, stderr string
}

func rocaBinary() (string, error) { return filepath.Abs("../../bin/roca") }
func acceptanceTempDir(prefix string) (string, error) {
	root, err := filepath.Abs("../../.tmp")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	return os.MkdirTemp(root, prefix)
}
func installPlayground(home string) error {
	binary, err := filepath.Abs("../../bin/roca-playground")
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".roca", "plugins", "roca-playground")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.Symlink(binary, filepath.Join(dir, "roca-playground"))
}
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

var _ = fmt.Sprint
