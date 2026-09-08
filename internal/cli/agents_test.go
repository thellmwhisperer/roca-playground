package cli

import (
	"strings"
	"testing"

	"github.com/thellmwhisperer/la-roca/internal/provider/service"
)

func TestDoctorExplainsTheZeroLoginFactorySelection(t *testing.T) {
	var output strings.Builder
	renderDoctor(&cliEnv{out: &output}, service.DoctorReport{
		DetectedModelBinaries:  []string{"claude", "codex"},
		FactoryDefault:         true,
		FactoryDefaultProvider: "claude",
	})
	out := output.String()
	for _, want := range []string{
		"model binaries detected: claude, codex",
		"factory default selected: claude",
		"confirm it with roca model check claude",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output does not contain %q:\n%s", want, out)
		}
	}
}
