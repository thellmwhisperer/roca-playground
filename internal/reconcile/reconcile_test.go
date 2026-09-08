package reconcile

import (
	"testing"

	"github.com/thellmwhisperer/la-roca/internal/provider/config"
)

func TestRetiredProviderChangesEveryDeclaredOrder(t *testing.T) {
	candidates := retiredProviderCandidates(Context{}, config.File{Models: config.ModelsConfig{
		ExploreOrder: []string{"retired"},
	}})
	if len(candidates) != 1 || candidates[0].Name != "retired" {
		t.Fatalf("provider declared only in explore_order was not detected: %+v", candidates)
	}
	changes := retiredProviderChanges("retired", "codex", nil)
	seen := map[string]bool{}
	for _, change := range changes {
		if change.Table == "models" && change.Kind == config.ReplaceListValue {
			seen[change.Key] = true
		}
	}
	for _, key := range []string{"order", "interpret_order", "explore_order"} {
		if !seen[key] {
			t.Errorf("retired provider remains in models.%s", key)
		}
	}
}
