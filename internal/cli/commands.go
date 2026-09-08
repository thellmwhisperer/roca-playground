package cli

import (
	"context"

	"time"

	"github.com/spf13/cobra"
	"github.com/thellmwhisperer/la-roca/internal/distribution/axi"

	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/service"

	_ "modernc.org/sqlite"
)

func playgroundCommand(env *cliEnv) *cobra.Command {
	var req service.QueryRequest
	var full bool
	var databases string
	cmd := &cobra.Command{
		Use:   "playground <question>",
		Short: "Human reading room: natural-language SQL and optional prose",
		Long: "Compile a question into SQL with the answering model and optionally explain the rows. " +
			"Agents search with `roca query`; this room is for humans. " +
			"Questions must contain text and may be at most 1000 characters.",
		Args: cobra.MinimumNArgs(1),
		RunE: scopedQuestionRunE(env, &req, &databases, func(cmd *cobra.Command, svc *service.Service) error {
			// The query may round-trip a model, and a model takes long enough to read
			// as frozen. The spinner says it is running on the error stream of an
			// interactive terminal only, so a piped call and a --json call see nothing.
			spin := startSpinner(env, spinnerShaping)
			live := newLiveInterpretation(env, spin, full, svc.DB().Path())
			req.Progress = queryProgress(spin)
			req.InterpretationStart = live.start
			req.InterpretationDelta = live.append
			answer, err := answerQuery(cmd.Context(), svc, req, full)
			spin.finish()
			if err != nil {
				return err
			}
			result := answer.result
			// A question that needed a model on a machine with no model
			// available is not an answer, even when the keyword rescue found
			// rows. The rows are a courtesy; the exit code tells the truth, so
			// a script does not read "it worked" from a machine that has
			// nothing to answer with.
			if printed, err := env.recordQueryResult(&result, svc); printed || err != nil {
				return err
			}
			if live.finish(answer) {
				return nil
			}
			env.print("database: %s", svc.DB().Path())
			if answer.interpretErr != nil {
				env.print("%s", interpretationFallback(answer.interpretErr))
			}
			answer.prose = formatInterpretation(answer.prose, termAware(env.out),
				terminalWidth(env.out), colorOn(env.out))
			env.print("%s", axiQuery(answer))
			return nil
		}),
	}
	cmd.Flags().StringVar(&req.Layer, "layer", "", "restrict the answer to one layer")
	cmd.Flags().IntVar(&req.MaxChars, "max-chars", service.DefaultMaxChars, "character budget per text field")
	cmd.Flags().BoolVar(&req.SQLOnly, "sql-only", false, "return the SQL without running it")
	cmd.Flags().BoolVar(&full, "full", false, "add a prose interpretation for human reading")
	addDatabaseFlag(cmd, &databases)
	return cmd
}

func exploreCommand(env *cliEnv) *cobra.Command {
	var req service.QueryRequest
	var deep bool
	var databases string
	cmd := &cobra.Command{
		Use:   "explore <term>",
		Short: "Investigate one concept through grounded memory",
		Long: "Investigate one concept with prose, deterministic terrain facts, and the generated SQL. " +
			"Use --deep for the full terrain map and 2-3 next probes.",
		Args: cobra.MinimumNArgs(1),
		RunE: scopedQuestionRunE(env, &req, &databases, func(cmd *cobra.Command, svc *service.Service) error {
			spin := startSpinner(env, spinnerShaping)
			req.Progress = queryProgress(spin)
			result, err := svc.Explore(cmd.Context(), service.ExploreRequest{
				QueryRequest: req, Deep: deep,
			})
			spin.finish()
			if err != nil {
				return err
			}
			if printed, err := env.recordQueryResult(&result, svc); printed || err != nil {
				return err
			}
			result.Interpretation = formatInterpretation(result.Interpretation, termAware(env.out),
				terminalWidth(env.out), colorOn(env.out))
			env.print("%s", axi.Explore(result))
			return nil
		}),
	}
	cmd.Flags().StringVar(&req.Layer, "layer", "", "restrict the investigation to one layer")
	cmd.Flags().IntVar(&req.MaxChars, "max-chars", service.DefaultMaxChars, "character budget per text field")
	cmd.Flags().BoolVar(&deep, "deep", false, "use the full terrain map and propose 2-3 next probes")
	addDatabaseFlag(cmd, &databases)
	return cmd
}

func scopedQuestionRunE(env *cliEnv, req *service.QueryRequest, databases *string,
	run func(*cobra.Command, *service.Service) error) func(*cobra.Command, []string) error {
	return env.serviceRunE(func(cmd *cobra.Command, args []string, svc *service.Service) error {
		if err := bindQuestionScope(req, args, *databases); err != nil {
			return err
		}
		return run(cmd, svc)
	})
}

func answerQuery(ctx context.Context, svc *service.Service, req service.QueryRequest,
	full bool) (queryAnswer, error) {
	result, err := svc.Query(ctx, req)
	answer := queryAnswer{result: result}
	if err != nil || !full || result.Engine == "" || result.RowCount == 0 {
		return answer, err
	}
	var interpretationMS int64
	if req.Progress != nil {
		req.Progress(service.QueryPhaseInterpretation)
	}
	var onStart func(bool)
	if req.InterpretationStart != nil {
		onStart = func(native bool) { req.InterpretationStart(native, result) }
	}
	firstOnStart, firstOnDelta, flushInterpretation :=
		service.BufferInterpretationCallbacks(onStart, req.InterpretationDelta)
	started := time.Now()
	interpretation, err := svc.InterpretStream(
		ctx, result.Question, result.Columns, result.Rows,
		time.Duration(result.SQLInferenceMS)*time.Millisecond,
		result.Engine, service.InterpretationContext{
			Mission: service.InterpretationAnswer, UnusedDatabases: result.UnusedDatabases,
		},
		firstOnStart, firstOnDelta)
	interpretationMS += time.Since(started).Milliseconds()
	if err == nil && service.CanWidenAfterInterpretation(result, interpretation.Text) {
		first := result
		req.Databases = []string{service.ScopeAll}
		widened, widenErr := svc.Query(ctx, req)
		if widenErr != nil {
			return queryAnswer{result: service.MergeWidenedResult(first, widened)}, widenErr
		}
		secondSQLInferenceMS := widened.SQLInferenceMS
		result = service.MergeWidenedResult(first, widened)
		interpretation = service.Interpretation{}
		err = nil
		if result.Engine != "" && result.RowCount > 0 {
			started = time.Now()
			interpretation, err = svc.InterpretStream(
				ctx, result.Question, result.Columns, result.Rows,
				time.Duration(secondSQLInferenceMS)*time.Millisecond,
				result.Engine, service.InterpretationContext{Mission: service.InterpretationAnswer},
				onStart, req.InterpretationDelta)
			interpretationMS += time.Since(started).Milliseconds()
		}
	} else if err == nil {
		flushInterpretation()
	}
	answer.result = result
	answer.prose, answer.interpretErr = interpretation.Text, err
	answer.result.InterpretationMS = interpretationMS
	answer.result.LatencyMS += answer.result.InterpretationMS
	answer.result.Interpretation = interpretation.Text
	// Who read the rows travels in the envelope beside who wrote the SQL: on an
	// installation that splits the two inferences they are different providers,
	// and that difference is the whole point of splitting them.
	answer.result.InterpretEngine = interpretation.Engine
	answer.result.InterpretModel = interpretation.Model
	answer.result.InterpretNote = interpretation.Note
	if answer.interpretErr != nil {
		answer.result.ProviderError = answer.interpretErr.Error()
	}
	return answer, nil
}
