package service

import (
	"context"
	core "github.com/thellmwhisperer/la-roca/internal/provider/service"
	"github.com/thellmwhisperer/la-roca/plugins/playground/internal/provider"
)

type Options struct {
	core.Options
	Providers, Interpreters, Explorers provider.Cascade
}
type Service struct {
	*core.Service
	opts Options
}

func Open(opts Options) (*Service, error) {
	inner, err := core.Open(opts.Options)
	if err != nil {
		return nil, err
	}
	return &Service{Service: inner, opts: opts}, nil
}

type QueryRequest = core.QueryRequest
type QueryResult = core.QueryResult
type QueryPhase = core.QueryPhase
type PluginRoute = core.PluginRoute
type Terrain = core.Terrain
type TerrainCount = core.TerrainCount
type DoctorProvider = core.DoctorProvider

var TextBudget = core.TextBudget
var QuestionRoute = core.QuestionRoute
var EnsureDatabaseColumn = core.EnsureDatabaseColumn
var SchemaWithPlugins = core.SchemaWithPlugins

func progress(req QueryRequest, phase QueryPhase) {
	if req.Progress != nil {
		req.Progress(phase)
	}
}
func (s *Service) Probe(ctx context.Context, report *core.DoctorReport) error {
	c := s.opts.Providers
	report.ModelDisabled, report.Warnings = c.Disabled, c.Warnings
	report.DetectedModelBinaries = c.DetectedBinaries
	report.MissingModelBinaries = provider.MissingCommandPresets(c.DetectedBinaries)
	report.FactoryDefault = c.FactoryDefault
	report.Providers, report.Titular = verdicts(ctx, c)
	if c.FactoryDefault {
		report.FactoryDefaultProvider = report.Titular
	}
	report.Interpreters, report.InterpretTitular = verdicts(ctx, s.opts.Interpreters)
	report.Explorers, report.ExploreTitular = verdicts(ctx, s.opts.Explorers)
	return nil
}

const (
	PathLLM                  = core.PathLLM
	PathKeyword              = core.PathKeyword
	PathUnresolved           = core.PathUnresolved
	PathRefused              = core.PathRefused
	PathAsk                  = core.PathAsk
	MatchFound               = core.MatchFound
	MatchEmpty               = core.MatchEmpty
	RetryGateRejection       = core.RetryGateRejection
	RetryExecutionError      = core.RetryExecutionError
	QueryPhaseSQL            = core.QueryPhaseSQL
	QueryPhaseExecution      = core.QueryPhaseExecution
	QueryPhaseInterpretation = core.QueryPhaseInterpretation
	DegradedUnavailable      = core.DegradedUnavailable
	DegradedLLMError         = core.DegradedLLMError
	DegradedInvalidSQL       = core.DegradedInvalidSQL
	DegradedExecution        = core.DegradedExecution
	DegradedTimeout          = core.DegradedTimeout
	ScopeAll                 = core.ScopeAll
	DefaultMaxChars          = core.DefaultMaxChars
)

var errQueryTimeout = core.ErrQueryTimeout
var truncate = core.Truncate

type InitResult = core.InitResult
type InitModel = core.InitModel

var ParseDatabaseList = core.ParseDatabaseList
var IsDegradedFailure = core.IsDegradedFailure

func Wrap(inner *core.Service, providers, interpreters, explorers provider.Cascade) *Service {
	return &Service{Service: inner, opts: Options{Options: inner.Options(), Providers: providers, Interpreters: interpreters, Explorers: explorers}}
}

type StoreRequest = core.StoreRequest
type ExecRequest = core.ExecRequest
type ExecResult = core.ExecResult

var DefaultQueryTimeout = core.DefaultQueryTimeout
var ScanRows = core.ScanRows

func (s *Service) modelGate(ctx context.Context) *InitModel {
	cascade := s.opts.Providers
	if cascade.Disabled {
		return &InitModel{Disabled: true, Reason: "the model is turned off in the configuration"}
	}
	if len(cascade.Providers) == 0 {
		return &InitModel{
			Reason: "no model provider is configured",
			Action: "declare one under [models] in " + s.opts.ConfigPath +
				", or run `roca doctor` to see the ones this version knows",
		}
	}
	gate := &InitModel{}
	for i, attempt := range cascade.Diagnose(ctx) {
		if attempt.Ready {
			transport, command := cascade.Providers[i].(interface{ CommandTransport() bool })
			return &InitModel{Ready: true, Provider: attempt.Name, Model: attempt.ModelID,
				CommandTransport: command && transport.CommandTransport()}
		}
		if gate.Reason == "" {
			gate.Provider, gate.Reason, gate.Action = attempt.Name, attempt.Reason, attempt.Action
		}
	}
	return gate
}
func (s *Service) Doctor(ctx context.Context) (core.DoctorReport, error) {
	report, err := s.Service.Doctor(ctx)
	if err != nil {
		return report, err
	}
	return report, s.Probe(ctx, &report)
}
func (s *Service) Init(ctx context.Context) (core.InitResult, error) {
	result, err := s.Service.Init(ctx)
	if err != nil {
		return result, err
	}
	result.Model = s.modelGate(ctx)
	return result, nil
}

type Authorship = core.Authorship

const SurfaceCLI = core.SurfaceCLI
const SurfaceMCP = core.SurfaceMCP

type DoctorReport = core.DoctorReport
type Bedrock = core.Bedrock
