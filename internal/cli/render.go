package cli

import (
	"context"

	"errors"

	"strings"

	"github.com/thellmwhisperer/la-roca/internal/distribution/axi"

	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/service"

	_ "modernc.org/sqlite"
)

func queryProgress(spin *spinner) func(service.QueryPhase) {
	return func(phase service.QueryPhase) {
		switch phase {
		case service.QueryPhaseExecution:
			spin.phase(spinnerSearching)
		case service.QueryPhaseInterpretation:
			spin.phase(spinnerComposing)
		default:
			spin.phase(spinnerShaping)
		}
	}
}

func bindQuestionScope(req *service.QueryRequest, args []string, databases string) error {
	req.Question = strings.Join(args, " ")
	names, err := service.ParseDatabaseList(databases)
	if err != nil {
		return err
	}
	req.Databases = names
	return nil
}

func (env *cliEnv) recordQueryResult(result *service.QueryResult,
	svc *service.Service) (bool, error) {
	env.auditQuery = result
	if service.IsDegradedFailure(result.Degraded) {
		env.code = ExitError
	}
	if !env.json {
		return false, nil
	}
	return true, env.printJSON(struct {
		service.QueryResult
		DatabasePath string `json:"database_path"`
	}{*result, svc.DB().Path()})
}

func interpretationFallback(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "summary timed out; showing rows instead."
	}
	return "summary unavailable; showing rows instead."
}

func axiQuery(answer queryAnswer) string {
	return axi.Query(answer.result, answer.prose)
}

type queryAnswer struct {
	result       service.QueryResult
	prose        string
	interpretErr error
}

func render(env *cliEnv, res service.QueryResult, prose string) {
	// The AXI text — route preamble, optional prose, rows and contextual help —
	// has one owner in the axi package.
	env.print("%s", axi.Query(res, prose))
}
