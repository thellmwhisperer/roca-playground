package cli

// @overview Exercise the public transport failure contract with isolated providers.
// READING GUIDE: Start at TestTransportPreservesProviderFailure, then argument errors.
// MAIN FLOW: synthetic home -> Execute -> decode streams -> assert exit and metadata.
// PUBLIC API: none; Go test discovers the regression cases.
// INTERNALS: provider failure and argument failure cases.
// @exports none
// @deps testing, encoding/json, strings; existing isolated CLI fixtures

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/service"
)

// -- 1/2 CORE · TestTransportPreservesProviderFailure <- START HERE --
func TestTransportPreservesProviderFailure(t *testing.T) {
	home := isolatedLoginHome(t)
	build := Build{Version: "test"}
	initializeProviderFixture(t, home, build)
	writeCommandProviderConfig(t, home, "unavailable", "synthetic-model", false)
	var out, diagnostics strings.Builder
	code, err := Execute(build, []string{"--transport", "playground", "synthetic sentinel", "--json"}, strings.NewReader(""), &out, &diagnostics)
	if err != nil || code != ExitError {
		t.Fatalf("code=%d err=%v stdout=%s stderr=%s", code, err, &out, &diagnostics)
	}
	var result service.QueryResult
	var envelope struct {
		Stderr string               `json:"stderr"`
		Query  *service.QueryResult `json:"query"`
	}
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(diagnostics.String()), &envelope); err != nil {
		t.Fatal(err)
	}
	if !service.IsDegradedFailure(result.Degraded) || envelope.Query == nil {
		t.Fatalf("missing failure metadata: stdout=%s stderr=%s", &out, &diagnostics)
	}
	audit := envelope.Query
	if audit.Degraded != result.Degraded || audit.Question != result.Question || len(audit.Providers) == 0 || len(audit.Rows) != 0 || len(audit.Columns) != 0 {
		t.Fatalf("failure audit differs from answer or contains rows: %s", &diagnostics)
	}
}

// -/ 1/2

// -- 2/2 HELPER · TestTransportArgumentErrorUsesOneEnvelope --
func TestTransportArgumentErrorUsesOneEnvelope(t *testing.T) {
	_ = isolatedLoginHome(t)
	var out, diagnostics strings.Builder
	code, err := Execute(Build{Version: "test"}, []string{"--transport", "playground"}, strings.NewReader(""), &out, &diagnostics)
	if code != ExitError || err != nil || out.Len() != 0 {
		t.Fatalf("code=%d err=%v stdout=%s", code, err, &out)
	}
	var envelope struct {
		Stderr string               `json:"stderr"`
		Query  *service.QueryResult `json:"query"`
	}
	if err := json.Unmarshal([]byte(diagnostics.String()), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Stderr == "" || envelope.Query != nil {
		t.Fatalf("argument error must contain diagnostics without invented audit: %s", &diagnostics)
	}
}

// -/ 2/2
