package cli

import (
	"encoding/json"
	"github.com/thellmwhisperer/la-roca/internal/store"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedCandidate(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Adopt(t.Context(), db, filepath.Join(filepath.Dir(path), "backups")); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func isolateRuntimeDirs(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("ROCA_DB_PATH", "")
	t.Setenv("ROCA_CONFIG", "")
	t.Setenv("ROCA_MODELS_ORDER", "none")
	for _, key := range []string{
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "CURSOR_HOME", "GROK_HOME", "OPENCODE_CONFIG",
		"HERMES_HOME", "PI_CODING_AGENT_DIR", "QWEN_HOME",
	} {
		t.Setenv(key, "")
	}
}

func runRoot(t *testing.T, build Build, args ...string) string {
	t.Helper()
	out, err := runRootErr(t, build, nil, args...)
	if err != nil {
		t.Fatalf("roca %v: %v", args, err)
	}
	return strings.TrimSpace(out)
}

func runRootErr(t *testing.T, build Build, in io.Reader, args ...string) (string, error) {
	t.Helper()
	var out strings.Builder
	env := hermeticCLIEnv(&cliEnv{build: build, out: &out, errOut: &out})
	root := rootCommand(env)
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&out)
	if in != nil {
		root.SetIn(in)
	}
	err := root.Execute()
	return out.String(), err
}

func mustJSON(t *testing.T, out string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output is not a JSON object: %v\n%s", err, out)
	}
	return doc
}

const coreMemoryFeatureConfig = "[features]\nplugins = true\nroca_ops = false\n"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
