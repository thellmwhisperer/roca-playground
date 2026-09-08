package cli

import (
	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/provider"

	"bufio"
	"fmt"
	"github.com/thellmwhisperer/la-roca/internal/provider/config"
	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/reconcile"
	"golang.org/x/term"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var terminalInput = func(in any) bool { f, ok := in.(*os.File); return ok && term.IsTerminal(int(f.Fd())) }

func (env *cliEnv) initSay(format string, args ...any) { fmt.Fprintf(env.errOut, format+"\n", args...) }
func (env *cliEnv) reconciliationContextFor(paths config.Paths) (reconcile.Context, error) {
	file, err := config.LoadFile(paths.Config)
	return reconcile.Context{Version: env.build.Version, ConfigPath: paths.Config, StampPath: paths.Reconciliation, File: file, Env: os.Getenv, LookPath: provider.LookPath, RetiredCredentialPaths: map[string]string{"codex": filepath.Join(filepath.Dir(paths.DB), "credentials", "codex.json")}}, err
}
func dirOf(path string) string { return filepath.Dir(path) }
func orDash(text string) string {
	if text == "" {
		return "-"
	}
	return text
}

func initMachineDuration(elapsed, promptWait time.Duration) time.Duration {
	if elapsed <= promptWait {
		return 0
	}
	return elapsed - promptWait
}

func (env *cliEnv) readInitRaw(reader *bufio.Reader) (string, error) {
	started := time.Now()
	line, err := reader.ReadString('\n')
	env.initPromptWait += time.Since(started)
	return line, err
}

func detectedAgentsLine(agents []string) string {
	if len(agents) == 0 {
		return "none"
	}
	return strings.Join(agents, ", ")
}
