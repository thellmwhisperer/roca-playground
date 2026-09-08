package service

import (
	"context"

	"slices"

	"time"

	"github.com/thellmwhisperer/la-roca/internal/provider/plugin"
	"github.com/thellmwhisperer/la-roca/internal/provider/query"
)

func (s *Service) Query(ctx context.Context, req QueryRequest) (res QueryResult, err error) {
	start := time.Now()
	req.MaxChars = TextBudget(req.MaxChars)
	if err := query.ValidateQuestion(req.Question, !s.opts.DisableStrictInput); err != nil {
		return res, err
	}
	res = QueryResult{
		Question:  req.Question,
		MaxChars:  req.MaxChars,
		Version:   s.opts.Version,
		SourceSHA: s.opts.Commit,
		// What the configuration said that this build did not understand travels
		// with every answer: a question is exactly where an operator would
		// otherwise never find out that half their [models] section is being
		// ignored.
		Warnings: slices.Clone(s.opts.Providers.Warnings),
	}
	defer func() { res.LatencyMS = time.Since(start).Milliseconds() }()
	if !s.opts.DisableMissingReferentAsk {
		if missing, Found := query.DetectMissingReferent(req.Question); Found {
			res.Path = PathAsk
			res.Message = missing.Ask
			res.ClarificationRequired = true
			res.MissingSlot = missing.Slot
			return res, nil
		}
	}
	if _, err := s.EnsureSchema(ctx); err != nil {
		return res, err
	}
	inventory := s.InventoryRoute(ctx)
	defer inventory.CloseOnDemand()
	route, err := QuestionRoute(req.Databases, inventory)
	if err != nil {
		return res, err
	}
	if s.PluginsActive() {
		res.Databases = route.Consulted()
	}
	res.OmittedDatabases = route.OmittedSources()
	res.UnusedDatabases = route.UnusedNames(inventory)
	res.Warnings = append(res.Warnings, route.Warnings...)
	res, err = s.llmStage(ctx, req, res, route)
	if err != nil {
		return res, err
	}
	if !req.SQLOnly && insufficientAnswer(res) && route.CanWiden(inventory) {
		widened := inventory
		res = beginWidenedPass(res, widened)
		res, err = s.llmStage(ctx, req, res, widened)
		if err != nil {
			return res, err
		}
		res.Widened = true
		res.Databases = widened.Consulted()
		res.UnusedDatabases = nil
	}
	if len(route.Databases) > 0 && !res.Widened && res.Path == PathKeyword &&
		!slices.Contains(res.Columns, plugin.ProvenanceColumn) && route.IncludeCore {
		res.Columns, res.Rows = EnsureDatabaseColumn(res.Columns, res.Rows, "core")
	}
	return res, err
}

func beginWidenedPass(first QueryResult, route PluginRoute) QueryResult {
	return QueryResult{
		Question:                  first.Question,
		MaxChars:                  first.MaxChars,
		Databases:                 route.Consulted(),
		OmittedDatabases:          route.OmittedSources(),
		Widened:                   true,
		RetriedSQL:                first.RetriedSQL,
		RetryType:                 first.RetryType,
		FirstModelSQL:             first.FirstModelSQL,
		RetryReason:               first.RetryReason,
		FirstRepaired:             slices.Clone(first.FirstRepaired),
		Providers:                 slices.Clone(first.Providers),
		Warnings:                  slices.Clone(first.Warnings),
		LLMLatencyMS:              first.LLMLatencyMS,
		SQLRetryProviderLatencyMS: first.SQLRetryProviderLatencyMS,
		SQLInferenceMS:            first.SQLInferenceMS,
		SQLRetryInferenceMS:       first.SQLRetryInferenceMS,
		ExecutionMS:               first.ExecutionMS,
		Version:                   first.Version,
		SourceSHA:                 first.SourceSHA,
	}
}

func MergeWidenedResult(first, widened QueryResult) QueryResult {
	widened.Widened = true
	widened.Providers = append(slices.Clone(first.Providers), widened.Providers...)
	widened.LLMLatencyMS += first.LLMLatencyMS
	widened.SQLRetryProviderLatencyMS += first.SQLRetryProviderLatencyMS
	widened.SQLInferenceMS += first.SQLInferenceMS
	widened.SQLRetryInferenceMS += first.SQLRetryInferenceMS
	widened.ExecutionMS += first.ExecutionMS
	widened.LatencyMS += first.LatencyMS
	if first.RetriedSQL && !widened.RetriedSQL {
		widened.RetriedSQL = true
		widened.RetryType = first.RetryType
		widened.FirstModelSQL = first.FirstModelSQL
		widened.RetryReason = first.RetryReason
		widened.FirstRepaired = slices.Clone(first.FirstRepaired)
	}
	return widened
}
