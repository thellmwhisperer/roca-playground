package cli

import (
	"fmt"

	"os"

	"strings"

	"github.com/thellmwhisperer/la-roca/internal/provider/config"
	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/provider"

	_ "modernc.org/sqlite"
)

func initModelChange(name, model, path string) string {
	file, _ := config.LoadFile(path)
	orderOverride := strings.TrimSpace(os.Getenv(provider.EnvOrder)) != ""
	modelOverrides := initModelEnvironmentOverrides(name, model, file)
	change := fmt.Sprintf("roca model set <id> or models.%s.model in %s", name, path)
	effectiveChange := change
	if orderOverride {
		effectiveChange = "models.<provider>.model in " + path
	}

	var governing, unset []string
	if orderOverride {
		governing = append(governing, provider.EnvOrder)
		unset = append(unset, provider.EnvOrder)
	}
	if len(modelOverrides) > 0 {
		governing = append(governing, modelOverrides[0])
		unset = append(unset, modelOverrides...)
	}
	guidance := effectiveChange
	if len(governing) > 0 {
		guidance = fmt.Sprintf("change %s directly; or unset %s before using %s",
			strings.Join(governing, " and "), strings.Join(unset, " and "), effectiveChange)
	}
	if orderOverride {
		guidance = change + "; " + guidance
	}
	if transport := initModelTransportOverride(name, path, file); transport != "" {
		guidance += "; transport is governed by " + transport +
			"; remove or change it to use the built-in transport"
	}
	return guidance + "; run roca doctor to confirm who will answer"
}

func initModelEnvironmentOverrides(name, model string, file config.File) []string {
	keys := map[string][]string{
		provider.NameCodex:  {"ROCA_CODEX_MODEL"},
		provider.NameOllama: {"ROCA_OLLAMA_MODEL", "ROCA_MODEL"},
	}[name]
	if name == provider.NameCodex && provider.UsesCommandTransport(file, name) ||
		name == provider.NameOllama && len(file.Models.Providers[name].Command) > 0 {
		return nil
	}
	var overrides []string
	for _, key := range keys {
		if os.Getenv(key) != "" {
			overrides = append(overrides, key)
		}
	}
	if len(overrides) > 0 && os.Getenv(overrides[0]) != model {
		return nil
	}
	return overrides
}

func initModelTransportOverride(name, path string, file config.File) string {
	cfg := file.Models.Providers[name]
	switch {
	case len(cfg.Command) > 0:
		return fmt.Sprintf("models.%s.command in %s", name, path)
	default:
		return ""
	}
}
