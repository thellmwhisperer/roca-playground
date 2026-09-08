package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"strings"
	"testing"

	"github.com/thellmwhisperer/la-roca/internal/provider/config"
	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/provider"
)

func TestInitOwnedModelChoiceWritesWithoutRecoveryBackup(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{DB: filepath.Join(root, "roca.db"), Config: filepath.Join(root, "config.toml")}
	features := "[features]\nplugins = true\nroca_ops = true\ncron = true\nvector = false\n"
	if err := os.WriteFile(paths.Config, []byte(features), 0o600); err != nil {
		t.Fatal(err)
	}
	outcome, err := writeInitModelChoice(paths, provider.NameCodex, "gpt-current")
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Changed || outcome.Backup != "" {
		t.Fatalf("init-owned model write outcome = %+v", outcome)
	}
	file, err := config.LoadFile(paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if file.Models.Providers[provider.NameCodex].Model != "gpt-current" ||
		!file.Features.Plugins || !file.Features.RocaOps || !file.Features.Cron || file.Features.Vector {
		t.Fatalf("init-owned config = %+v", file)
	}
	if _, err := os.Stat(paths.Config + ".roca.bak"); !os.IsNotExist(err) {
		t.Fatalf("init-owned model write created an operator recovery backup: %v", err)
	}
}

type chooserTestBackend struct {
	catalogues map[string]modelCatalogue
}

func (b chooserTestBackend) Catalogue(_ context.Context, name, _ string) (modelCatalogue, error) {
	if catalogue, ok := b.catalogues[name]; ok {
		return catalogue, nil
	}
	return modelCatalogue{}, fmt.Errorf("no enumerable catalogue")
}

func (chooserTestBackend) Probe(context.Context, string, string) error { return nil }

func runExplicitNonTTYInit(t *testing.T, home string) (string, string) {
	t.Helper()
	dbPath := filepath.Join(home, "explicit", "roca.db")
	out, err := runInitChooser(t, false, "", chooserTestBackend{},
		"init", "--db-path", dbPath)
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	return out, filepath.Join(filepath.Dir(dbPath), "config.toml")
}

func assertInitFeatures(t *testing.T, configPath, label string) {
	t.Helper()
	file, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !file.Features.Plugins || !file.Features.RocaOps || !file.Features.Cron || file.Features.Vector {
		t.Fatalf("%s config has wrong features: %+v", label, file.Features)
	}
}

func initChooserHome(t *testing.T) (string, string) {
	t.Helper()
	home, bin := t.TempDir(), t.TempDir()
	isolateRuntimeDirs(t, home)
	t.Setenv("PATH", bin)
	t.Setenv("ROCA_DB_PATH", "")
	t.Setenv("ROCA_CONFIG", "")
	t.Setenv("ROCA_MODELS_ORDER", "")
	t.Setenv("ROCA_CODEX_MODEL", "")
	t.Setenv("ROCA_OLLAMA_MODEL", "")
	t.Setenv("ROCA_MODEL", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_, _ = w.Write([]byte(`{"models":[{"name":"local-one"},{"name":"environment-model"},{"name":"local-fallback"}]}`))
		case "/api/chat":
			_, _ = w.Write([]byte(`{"message":{"content":"SELECT 1"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("ROCA_OLLAMA_BASE_URL", server.URL)
	return home, bin
}

func fakeModelCLI(t *testing.T, bin, name string) {
	t.Helper()
	body := "#!/bin/sh\nprintf 'SELECT 1\\n'\n"
	if name == provider.NameClaude {
		body = "#!/bin/sh\nprintf '{\"result\":\"SELECT 1\"}\\n'\n"
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
}

func runInitChooser(t *testing.T, tty bool, input string, backend modelValidationBackend,
	args ...string) (string, error) {
	return runInitChooserReader(t, tty, strings.NewReader(input), backend, args...)
}

func runInitChooserReader(t *testing.T, tty bool, input io.Reader, backend modelValidationBackend,
	args ...string) (string, error) {
	t.Helper()
	previous := terminalInput
	terminalInput = func(any) bool { return tty }
	t.Cleanup(func() { terminalInput = previous })
	var out strings.Builder
	env := hermeticCLIEnv(&cliEnv{
		build: Build{Version: "test", Commit: "test-sha"}, out: &out, errOut: &out,
	})
	env.skipInitChooser = false
	env.skipReconciliation = false
	env.modelBackend = backend
	_, err := executeWithEnv(env, args, input)
	return out.String(), err
}

type firstReadHook struct {
	reader io.Reader
	hook   func()
}

func (reader *firstReadHook) Read(buffer []byte) (int, error) {
	if reader.hook != nil {
		hook := reader.hook
		reader.hook = nil
		hook()
	}
	return reader.reader.Read(buffer)
}

func countLinesWithPrefix(text, prefix string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count
}
