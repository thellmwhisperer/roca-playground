package service

import (
	"testing"

	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/provider"
)

func TestWidenReplyRequiresTheExactUppercaseToken(t *testing.T) {
	for _, tc := range []struct {
		reply string
		want  bool
	}{
		{reply: "WIDEN", want: true},
		{reply: "  WIDEN\n", want: true},
		{reply: "widen"},
		{reply: "Widen"},
		{reply: "WIDEN now"},
	} {
		if got := WidenReply(tc.reply); got != tc.want {
			t.Errorf("WidenReply(%q) = %v, want %v", tc.reply, got, tc.want)
		}
	}
}

func TestOnlyEmptyUsableAnswersMayWiden(t *testing.T) {
	for _, tc := range []struct {
		name string
		res  QueryResult
		want bool
	}{
		{name: "empty rows", res: QueryResult{Path: PathLLM, Match: MatchEmpty}, want: true},
		{name: "model unavailable with empty rescue", res: QueryResult{
			Path: PathKeyword, Match: MatchEmpty, Degraded: DegradedUnavailable,
		}, want: true},
		{name: "invalid sql", res: QueryResult{
			Path: PathKeyword, Match: MatchEmpty, Degraded: DegradedInvalidSQL,
		}},
		{name: "execution failure", res: QueryResult{
			Path: PathKeyword, Match: MatchEmpty, Degraded: DegradedExecution,
		}},
		{name: "execution timeout", res: QueryResult{
			Path: PathKeyword, Match: MatchEmpty, Degraded: DegradedTimeout,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := insufficientAnswer(tc.res); got != tc.want {
				t.Fatalf("insufficientAnswer(%+v) = %v, want %v", tc.res, got, tc.want)
			}
		})
	}
}

func TestWidenedPassKeepsOnlyCumulativeState(t *testing.T) {
	first := QueryResult{
		Question: "synthetic question", Path: PathKeyword, Message: "nothing relevant",
		Degraded: DegradedUnavailable, Match: MatchEmpty, Columns: []string{"text"},
		Rows: []map[string]any{{"text": "stale"}}, RowCount: 1,
		Providers: []provider.Attempt{{Name: "first"}}, Warnings: []string{"warning"},
		RetriedSQL: true, RetryType: RetryGateRejection, FirstModelSQL: "SELECT missing",
		RetryReason: "missing", FirstRepaired: []string{"code_fence"},
		LLMLatencyMS: 2, SQLRetryProviderLatencyMS: 1, SQLInferenceMS: 3,
		SQLRetryInferenceMS: 1, ExecutionMS: 4, Version: "v-test", SourceSHA: "abc",
	}
	got := beginWidenedPass(first, PluginRoute{IncludeCore: true})
	if got.Message != "" || got.Degraded != "" || got.Path != "" || got.Match != "" ||
		got.RowCount != 0 || got.Rows != nil || got.Columns != nil {
		t.Fatalf("widened pass retained stale answer state: %+v", got)
	}
	if !got.Widened || got.LLMLatencyMS != 2 || got.ExecutionMS != 4 ||
		len(got.Providers) != 1 || got.FirstModelSQL != "SELECT missing" {
		t.Fatalf("widened pass lost cumulative state: %+v", got)
	}
}

func TestMergeWidenedResultAccumulatesQueryTelemetry(t *testing.T) {
	first := QueryResult{
		Providers: []provider.Attempt{{Name: "first"}}, LLMLatencyMS: 2,
		SQLRetryProviderLatencyMS: 3, SQLInferenceMS: 5, SQLRetryInferenceMS: 7,
		ExecutionMS: 11, LatencyMS: 13, RetriedSQL: true, RetryType: RetryGateRejection,
		FirstModelSQL: "SELECT missing", RetryReason: "missing",
		FirstRepaired: []string{"code_fence"},
	}
	widened := QueryResult{
		Providers: []provider.Attempt{{Name: "second"}}, LLMLatencyMS: 17,
		SQLRetryProviderLatencyMS: 19, SQLInferenceMS: 23, SQLRetryInferenceMS: 29,
		ExecutionMS: 31, LatencyMS: 37,
	}
	got := MergeWidenedResult(first, widened)
	if !got.Widened || len(got.Providers) != 2 || got.LLMLatencyMS != 19 ||
		got.SQLRetryProviderLatencyMS != 22 || got.SQLInferenceMS != 28 ||
		got.SQLRetryInferenceMS != 36 || got.ExecutionMS != 42 || got.LatencyMS != 50 ||
		!got.RetriedSQL || got.FirstModelSQL != "SELECT missing" {
		t.Fatalf("merged telemetry = %+v", got)
	}
}
