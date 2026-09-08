package cli

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	corecli "github.com/thellmwhisperer/la-roca/internal/distribution/cli"
	"github.com/thellmwhisperer/la-roca/internal/provider/config"
	core "github.com/thellmwhisperer/la-roca/internal/provider/service"
	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/provider"
	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/reconcile"
	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/service"
	"io"
	"os"
	"strings"
	"time"
)

type Build = corecli.Build

const ExitError = 1

type cliEnv struct {
	build                                                    Build
	out, errOut                                              io.Writer
	dbPath                                                   string
	json, forceReadOnly, skipReconciliation, skipInitChooser bool
	code                                                     int
	auditQuery                                               *service.QueryResult
	modelBackend                                             modelValidationBackend
	modelPicker                                              modelPicker
	initPromptWait                                           time.Duration
}

func Execute(build Build, args []string, in io.Reader, out, errOut io.Writer) (int, error) {
	env := &cliEnv{build: build, out: out, errOut: errOut}
	root := rootCommand(env)
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs(args)
	err := root.Execute()
	if err != nil {
		return 1, err
	}
	return env.code, nil
}
func (env *cliEnv) print(format string, args ...any) { fmt.Fprintf(env.out, format+"\n", args...) }
func (env *cliEnv) printJSON(value any) error {
	encoder := json.NewEncoder(env.out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
func (env *cliEnv) resolvePaths() (config.Paths, error) {
	home, _ := os.UserHomeDir()
	return config.Resolve(config.Input{Flag: env.dbPath, Env: os.Getenv(config.EnvDBPath), Home: home, ConfigEnv: os.Getenv(config.EnvConfig)})
}
func (env *cliEnv) openService() (*service.Service, error) {
	inner, paths, err := corecli.OpenForPlugin(env.build, env.dbPath, env.forceReadOnly, env.out, env.errOut)
	if err != nil {
		return nil, err
	}
	file, err := config.LoadFile(paths.Config)
	if err != nil {
		inner.Close()
		return nil, err
	}
	a, b, c := buildProviders(file, paths)
	return service.Wrap(inner, a, b, c), nil
}
func (env *cliEnv) serviceRunE(run func(*cobra.Command, []string, *service.Service) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		svc, err := env.openService()
		if err != nil {
			return err
		}
		defer svc.Close()
		return run(cmd, args, svc)
	}
}
func addDatabaseFlag(cmd *cobra.Command, value *string) {
	cmd.Flags().StringVar(value, "databases", "", "comma-separated database names or all")
}
func buildProviders(file config.File, paths config.Paths) (provider.Cascade, provider.Cascade, provider.Cascade) {
	settings := provider.Settings{File: file, RunnerDir: paths.Runner, Env: os.Getenv}
	a, err := provider.BuildCascade(settings)
	if err != nil {
		a.Warnings = append(a.Warnings, err.Error())
	}
	b, err := provider.BuildInterpretCascade(settings)
	if err != nil {
		a.Warnings = append(a.Warnings, err.Error())
	}
	c, err := provider.BuildExploreCascade(settings)
	if err != nil {
		a.Warnings = append(a.Warnings, err.Error())
	}
	return a, b, c
}

func rootCommand(env *cliEnv) *cobra.Command {
	root := &cobra.Command{Use: "roca-playground", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringVar(&env.dbPath, "db-path", "", "database to use")
	root.PersistentFlags().BoolVar(&env.json, "json", false, "JSON output")
	root.PersistentFlags().BoolVar(&env.forceReadOnly, "read-only", false, "refuse writes")
	root.AddCommand(&cobra.Command{Use: "capabilities", Hidden: true, RunE: func(cmd *cobra.Command, _ []string) error {
		paths, err := env.resolvePaths()
		if err != nil {
			return err
		}
		ctx, err := env.reconciliationContextFor(paths)
		if err != nil {
			return err
		}
		return env.printJSON(reconcile.Open(ctx, reconcile.Registry()))
	}})
	root.AddCommand(playgroundCommand(env), exploreCommand(env), modelCommand(env), modelsCommand(env), loginCommand(env), &cobra.Command{Use: "probe", RunE: func(cmd *cobra.Command, _ []string) error {
		svc, err := env.openService()
		if err != nil {
			return err
		}
		defer svc.Close()
		var report core.DoctorReport
		if err := svc.Probe(cmd.Context(), &report); err != nil {
			return err
		}
		report.ConfigPath = svc.Options().ConfigPath
		var narration strings.Builder
		renderProviderDiagnosis(&cliEnv{out: &narration}, report)
		report.ProviderNarration = narration.String()
		return env.printJSON(report)
	}})

	return root
}
func executeWithEnv(env *cliEnv, args []string, in io.Reader) (int, error) {
	root := rootCommand(env)
	root.SetArgs(args)
	root.SetIn(in)
	err := root.Execute()
	if err != nil {
		return 1, err
	}
	return env.code, nil
}
func execute(build Build, out, errOut io.Writer, args []string) (int, error) {
	return Execute(build, args, os.Stdin, out, errOut)
}
