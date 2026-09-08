// Package reconcile notices capabilities a newer binary can use but the
// operator's existing configuration does not yet request.
package reconcile

import (
	core "github.com/thellmwhisperer/la-roca/internal/distribution/reconcile"

	"fmt"

	"os"

	"slices"
	"sort"
	"strings"

	"github.com/thellmwhisperer/la-roca/internal/provider/config"

	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/provider"
)

type Entry = core.Entry
type Detection = core.Detection
type Proposal = core.Proposal
type Context = core.Context
type Options = core.Options
type Result = core.Result

const ProposalClaudeCLI = core.ProposalClaudeCLI
const ProposalRetiredProvider = core.ProposalRetiredProvider
const ProviderUnavailable = core.ProviderUnavailable
const inputValue = "{operator-input}"

func Registry() []Entry {
	return []Entry{
		{
			ID: ProposalClaudeCLI,
			Detection: Detection{Binary: provider.NameClaude, Provider: provider.NameClaude,
				ProviderState: ProviderUnavailable},
			Proposal: Proposal{
				Alert:  "Claude Code is on PATH but no usable Claude provider is configured; model sonnet can answer through the existing local CLI session.",
				Prompt: "Enable the Claude provider?",
				Changes: []config.Change{{Kind: config.PrependUnique, Table: "models", Key: "order",
					Value: provider.NameClaude, Default: provider.DefaultOrder(nil)},
					{Kind: config.ReplaceTable, Table: "models.claude"}},
			},
		},
		{ID: ProposalRetiredProvider, Detection: Detection{RetiredProvider: true}},
	}
}

func Open(context Context, registry []Entry) []Entry {
	file := context.File
	if file.Path == "" && context.ConfigPath != "" {
		loaded, err := config.LoadFile(context.ConfigPath)
		if err != nil {
			return nil
		}
		file = loaded
	}
	var open []Entry
	for _, entry := range registry {
		if entry.ID == ProposalRetiredProvider && entry.Detection.RetiredProvider {
			open = append(open, retiredProviderEntries(context, file)...)
			continue
		}
		if detected(context, file, entry.Detection) {
			open = append(open, entry)
		}
	}
	return open
}

func detected(context Context, file config.File, detection Detection) bool {
	if detection.Capability != "" && !context.Capabilities[detection.Capability] {
		return false
	}
	if detection.Binary != "" && !binaryOnPath(context, detection.Binary) {
		return false
	}
	if detection.DefaultListEmpty != "" && len(file.DefaultList(detection.DefaultListEmpty)) > 0 {
		return false
	}
	if detection.Provider != "" {
		order := file.Models.Order
		if order == nil {
			order = provider.DefaultOrder(context.LookPath)
		}
		declared := slices.Contains(order, detection.Provider)
		switch detection.ProviderState {
		case ProviderUnavailable:
			if providerUsable(context, file, detection.Provider, declared) {
				return false
			}
		}
	}
	return true
}

func binaryOnPath(context Context, name string) bool {
	return provider.BinaryOnPath(context.LookPath, name)
}

func providerUsable(context Context, file config.File, name string, declared bool) bool {
	if !declared {
		return false
	}
	if provider.UsesCommandTransport(file, name) {
		command := file.Models.Providers[name].Command
		if len(command) == 0 {
			command = []string{name}
		}
		return binaryOnPath(context, command[0])
	}
	return false
}

func retiredProviderEntries(context Context, file config.File) []Entry {
	candidates := retiredProviderCandidates(context, file)
	detected := provider.DetectedCommandPresets(context.LookPath)
	entries := make([]Entry, 0, len(candidates))
	for _, candidate := range candidates {
		name := candidate.Name
		configured, declared := providerConfiguration(file, name)
		// An explicit command declaration is the operator's own transport. It is
		// never a retired artifact, so it survives every proposal below.
		ownCommand := declared && len(configured.Command) > 0
		if !candidate.RetireConfiguration {
			alert := fmt.Sprintf("Retired provider credential file detected for %s; no model configuration changes are needed.", name)
			if ownCommand {
				alert = fmt.Sprintf("Retired provider credential file detected for %s; its configured command transport remains unchanged.", name)
			}
			entries = append(entries, Entry{
				ID: ProposalRetiredProvider + "-credential-" + name,
				Proposal: Proposal{
					Alert:  alert,
					Prompt: fmt.Sprintf("Remove the retired %s credential file?", name),
				},
				RetiredProvider: name,
			})
			continue
		}
		target := ""
		if ownCommand || slices.Contains(detected, name) {
			target = name
		} else if len(detected) > 0 {
			target = detected[0]
		}
		changes := retiredProviderChanges(name, target, candidate.Tables)
		entry := Entry{ID: ProposalRetiredProvider + "-" + name,
			Proposal: Proposal{Changes: changes}, RetiredProvider: name}
		switch target {
		case "":
			entry.Proposal.Alert = fmt.Sprintf("Retired credential-backed model provider detected: drop %s from the provider order; no local agent CLI is on PATH.", name)
			entry.Proposal.Prompt = fmt.Sprintf("Drop %s from the model configuration?", name)
		case name:
			entry.Proposal.Alert = fmt.Sprintf("Retired credential-backed model provider detected: remove the retired %s authentication settings and keep the rest of its table; %s authenticates through its own CLI.", name, name)
			entry.Proposal.Prompt = fmt.Sprintf("Remove the retired %s authentication settings?", name)
		default:
			entry.Proposal.Alert = fmt.Sprintf("Retired credential-backed model provider detected: migrate %s to %s, which authenticates through its own CLI.", name, target)
			entry.Proposal.Prompt = fmt.Sprintf("Migrate %s to the %s local CLI?", name, target)
		}
		entries = append(entries, entry)
	}
	return entries
}

type retiredProviderCandidate struct {
	Name                string
	Tables              []string
	RetireConfiguration bool
}

func retiredProviderCandidates(context Context, file config.File) []retiredProviderCandidate {
	candidates := map[string]retiredProviderCandidate{}
	add := func(name, table string) {
		normalized := normalizeProviderName(name)
		if normalized == "" || !RetiredProviderConfiguration(file, name) {
			return
		}
		candidate := candidates[normalized]
		candidate.Name = normalized
		candidate.RetireConfiguration = true
		if table != "" && !slices.Contains(candidate.Tables, table) {
			candidate.Tables = append(candidate.Tables, table)
		}
		candidates[normalized] = candidate
	}
	declared := append(append(append([]string(nil), file.Models.Order...),
		file.Models.InterpretOrder...), file.Models.ExploreOrder...)
	for _, name := range declared {
		add(name, "")
	}
	for name, configured := range file.Models.Providers {
		table := configured.TableName
		if table == "" {
			table = name
		}
		add(name, table)
	}
	for name, path := range context.RetiredCredentialPaths {
		if regularFile(path) {
			normalized := normalizeProviderName(name)
			if normalized == "" {
				continue
			}
			candidate := candidates[normalized]
			candidate.Name = normalized
			candidates[normalized] = candidate
		}
	}
	ordered := make([]retiredProviderCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		sort.Strings(candidate.Tables)
		ordered = append(ordered, candidate)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Name < ordered[j].Name })
	return ordered
}

// RetiredProviderConfiguration reports whether the operator's own configuration
// still asks for a transport this build retired. A credential file left on disk
// by an older release is not part of that answer: it is a stale artifact with
// its own cleanup proposal, and it never disables a transport that works.
func RetiredProviderConfiguration(file config.File, name string) bool {
	normalized := normalizeProviderName(name)
	if normalized == "" || normalized == provider.NameOllama {
		return false
	}
	cfg, declared := providerConfiguration(file, name)
	if len(cfg.Command) > 0 || provider.UsesCommandTransport(file, normalized) {
		return declared && (cfg.BaseURL != "" || cfg.RetiredCredential)
	}
	return true
}

func providerConfiguration(file config.File, name string) (config.ProviderConfig, bool) {
	if configured, declared := file.Models.Providers[strings.ToLower(strings.TrimSpace(name))]; declared {
		return configured, true
	}
	normalized := normalizeProviderName(name)
	for candidate, configured := range file.Models.Providers {
		if normalizeProviderName(candidate) == normalized {
			return configured, true
		}
	}
	return config.ProviderConfig{}, false
}

func normalizeProviderName(name string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), "_", "-")
}

func regularFile(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func retiredProviderChanges(name, target string, tables []string) []config.Change {
	kind := config.RemoveListValue
	if target != "" {
		kind = config.ReplaceListValue
	}
	changes := []config.Change{
		{Kind: kind, Table: "models", Key: "order", Old: name, Value: target},
		{Kind: kind, Table: "models", Key: "interpret_order", Old: name, Value: target},
		{Kind: kind, Table: "models", Key: "explore_order", Old: name, Value: target},
	}
	if target == name {
		for _, table := range tables {
			for _, key := range []string{"base_url", "api_key", "api_key_env", "preset"} {
				changes = append(changes, config.Change{Kind: config.DeleteValue, Table: "models." + table, Key: key})
			}
		}
	} else {
		for _, table := range tables {
			changes = append(changes, config.Change{Kind: config.DeleteTable, Table: "models." + table})
		}
	}
	return changes
}

func Run(ctx Context, registry []Entry, options Options) (Result, error) {
	entries := Open(ctx, registry)
	for i := range entries {
		entries[i].Detection = Detection{}
	}
	return core.Run(ctx, entries, options)
}

var RedactRecoveryBackups = core.RedactRecoveryBackups
var RemoveRetiredCredential = core.RemoveRetiredCredential
