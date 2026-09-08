package cli

import (
	"time"

	"github.com/thellmwhisperer/la-roca/internal/ingest"

	"fmt"

	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/service"
)

func renderDoctor(env *cliEnv, report service.DoctorReport) {
	env.print("roca %s (%s)", report.Version, report.SourceSHA)
	env.print("database: %s · %d memories", report.DBPath, report.Memories)
	renderBedrock(env, report.Bedrock)
	if report.ConfigExists {
		env.print("configuration: %s", report.ConfigPath)
	} else {
		env.print("configuration: %s (does not exist: defaults in use)", report.ConfigPath)
	}
	env.print("agents detected: %s", detectedAgentsLine(report.DetectedAgents))
	env.print("agents not found: %s", missingAgentsLine(report.DetectedAgents))
	renderModelDetection(env, report.DetectedModelBinaries, report.MissingModelBinaries,
		report.FactoryDefault, report.FactoryDefaultProvider)
	env.print("authentication: local agent models use their own CLI sessions; La Roca stores no secrets")

	for _, warning := range report.Warnings {
		env.print("warning: %s", warning)
	}
	if len(report.LayerRepairs) > 0 {
		env.print("runtime_layers_not_in_registry: failed")
		for _, command := range report.LayerRepairs {
			env.print("      remedy: run `%s`", command)
		}
	}

	renderProviderDiagnosis(env, report)

	if report.PromptPath != "" {
		if report.PromptExists {
			env.print("agent prompt: %s (paste it into agent instructions)", report.PromptPath)
		} else {
			env.print("agent prompt: missing at %s", report.PromptPath)
			env.print("      remedy: run `roca init` to generate it")
		}
	}
	if len(report.CapabilityProposals) > 0 {
		env.print("open capability proposals:")
		for _, proposal := range report.CapabilityProposals {
			env.print("  - %s", proposal)
		}
	}
}

func renderProviders(env *cliEnv, configPath string, providers []service.DoctorProvider) {
	for _, p := range providers {
		status := "working"
		if !p.Ready {
			status = "failed"
		}
		env.print("  %s %s · model %s (%s · change with: %s) · probe %s",
			env.mark(p.Ready), p.Name, orDash(p.Model), modelChoiceSource(configPath, p.Name, p.Model),
			modelChange(p.Name, configPath), status)
		if !p.Ready {
			env.print("      %s", orDash(p.Reason))
			if p.Action != "" {
				env.print("      remedy: %s", p.Action)
			}
		}
	}
}

func renderInterpretation(env *cliEnv, report service.DoctorReport) {
	if len(report.Interpreters) == 0 {
		return
	}
	env.print("interpretation providers, in the declared order:")
	renderProviders(env, report.ConfigPath, report.Interpreters)
	if report.InterpretTitular != "" {
		env.print("the one that is going to read the result rows: %s "+
			"(the rows go to it and to no other provider)", report.InterpretTitular)
		return
	}
	env.print("no interpretation provider is available: the result rows fall back to " +
		"the provider that writes the SQL")
}

func renderExploration(env *cliEnv, report service.DoctorReport) {
	if len(report.Explorers) == 0 {
		return
	}
	env.print("deep exploration providers, in the declared order:")
	renderProviders(env, report.ConfigPath, report.Explorers)
	if report.ExploreTitular != "" {
		env.print("the one that is going to read deep exploration rows: %s", report.ExploreTitular)
		return
	}
	env.print("no deep exploration provider is available: deep mode falls back to " +
		"interpretation order, then main order")
}
func renderInitAnswer(env *cliEnv, result service.InitResult) {
	model := result.Model
	if model == nil || !model.Ready {
		env.print("answering: none · configuration: %s · change with: models.order and models.<provider>.model in that file; run roca doctor to confirm who will answer",
			result.ConfigPath)
		return
	}
	line := fmt.Sprintf("answering: %s/%s (%s) · configuration: %s",
		model.Provider, model.Model, modelChoiceSource(result.ConfigPath, model.Provider, model.Model),
		result.ConfigPath)
	if model.CommandTransport {
		line += " · uses the existing local CLI session; confirm it with roca model check"
	}
	line += " · change with: " + initModelChange(model.Provider, model.Model, result.ConfigPath)
	env.print("%s", line)
}

func renderBootstrap(env *cliEnv, result service.InitResult) {
	renderModelDetection(env, result.DetectedModelBinaries, result.MissingModelBinaries, result.FactoryDefault, result.FactoryDefaultProvider)
	renderInitAnswer(env, result)
}

func renderBedrock(env *cliEnv, bedrock *service.Bedrock) {
	if bedrock == nil {
		env.print("bedrock: your memory has no history yet")
		return
	}
	stamp, err := time.Parse(time.RFC3339, bedrock.Timestamp)
	if err != nil {
		stamp, err = time.Parse("2006-01-02 15:04:05", bedrock.Timestamp)
	}
	date := bedrock.Timestamp
	if err == nil {
		date = stamp.Format("02 Jan 2006")
	}
	if bedrock.Project != "" {
		env.print("bedrock: your memory reaches back to %s (first session: %s)", date, bedrock.Project)
		return
	}
	env.print("bedrock: your memory reaches back to %s", date)
}

func missingAgentsLine(detected []string) string {
	return detectedAgentsLine(ingest.MissingAgentFamilies(detected))
}

func renderProviderDiagnosis(env *cliEnv, report service.DoctorReport) {
	switch {
	case report.ModelDisabled && len(report.Providers) == 0:
		env.print("model: turned off by configuration")
	case len(report.Providers) == 0:
		env.print("model: no provider declared")
	default:
		env.print("providers, in the declared order:")
		renderProviders(env, report.ConfigPath, report.Providers)
	}

	if report.Titular != "" {
		env.print("the one that is going to answer: %s", report.Titular)
	} else if len(report.Providers) > 0 {
		env.print("no provider is available: questions the compiler does not resolve " +
			"will fall to the keyword rescue")
	}
	renderInterpretation(env, report)
	renderExploration(env, report)
}
