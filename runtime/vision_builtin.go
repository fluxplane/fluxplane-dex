package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/vision"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

const (
	visionPluginName        = "vision"
	visionOperationAnalyze  = "vision.analyze"
	visionOperationProvider = "vision.provider.list"
	visionFanoutConcurrency = 3
)

type visionNoInput struct{}

func visionManifest() core.PluginManifest {
	return pluginbinding.Manifest(visionManifestSpec())
}

func visionManifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        visionPluginName,
		Version:     "0.8.0",
		Description: "Generic image understanding aggregator over vision provider plugins.",
		Aliases:     []string{"image-vision", "vision"},
		Operations: []core.OperationSpec{
			visionProviderSpec(),
			visionAnalyzeSpec(),
		},
		Context: []core.ContextSpec{
			pluginbinding.ContextSpec("vision.context", "Image vision provider context.", pluginbinding.ContextKindText, pluginbinding.ContextKindReference),
		},
		Metadata: map[string]string{"kind": "builtin"},
	}
}

type visionBuiltinService struct {
	runner Runner
	ctx    context.Context
}

func (r Runner) visionPlugin(ctx context.Context) *pluginbinding.Plugin {
	if ctx == nil {
		ctx = context.Background()
	}
	service := visionBuiltinService{runner: r, ctx: ctx}
	return pluginbinding.Define(visionManifestSpec(),
		pluginbinding.RegisterOperation(visionProviderSpec(), service.Providers),
		pluginbinding.RegisterOperation(visionAnalyzeSpec(), service.Analyze),
		pluginbinding.RegisterContextProvider(pluginbinding.ContextSpec("vision.context", "Image vision provider context.", pluginbinding.ContextKindText, pluginbinding.ContextKindReference), service.Context),
	)
}

func visionProviderSpec() core.OperationSpec {
	return visionReadOperation[visionNoInput, vision.ProviderListResult](visionOperationProvider, "List available image vision provider plugins.")
}

func visionAnalyzeSpec() core.OperationSpec {
	return visionReadOperation[vision.AnalyzeInput, vision.AnalyzeOutput](visionOperationAnalyze, "Analyze one or more images through vision provider plugins.")
}

func visionReadOperation[I any, O any](name, description string) core.OperationSpec {
	return pluginbinding.TypedOperationSpec[I, O](
		name,
		description,
		pluginbinding.ReadOnly(),
		pluginbinding.Compact(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	)
}

func (s visionBuiltinService) Providers(pluginbinding.Context, visionNoInput) (vision.ProviderListResult, error) {
	providers := s.runner.visionProviders(s.ctx)
	return vision.ProviderListResult{Providers: providers, Count: len(providers)}, nil
}

func (s visionBuiltinService) Context(_ pluginbinding.Context, input pluginbinding.ContextBuildInput) (pluginbinding.ContextBuildResult, error) {
	providers := s.runner.visionProviders(s.ctx)
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		names = append(names, provider.Name)
	}
	content := "Vision aggregates image-understanding provider plugins through provider discovery."
	if len(names) > 0 {
		content += " Available providers: " + strings.Join(names, ", ") + "."
	} else {
		content += " No provider plugins are currently available."
	}
	if query := strings.TrimSpace(input.Query); query != "" {
		content += " Query: " + query + "."
	}
	return pluginbinding.ContextBuildResult{
		Blocks: []core.ContextBlock{{
			ID:       "vision.context",
			Kind:     pluginbinding.ContextKindText,
			Title:    "Vision context",
			Content:  content,
			Priority: 30,
			Metadata: map[string]string{
				"providers": strings.Join(names, ","),
				"operation": visionOperationAnalyze,
			},
		}},
	}, nil
}

func (s visionBuiltinService) Analyze(ctx pluginbinding.Context, input vision.AnalyzeInput) (vision.AnalyzeOutput, error) {
	output := s.runner.runVision(s.ctx, ctx.Request.Instance, input)
	if len(output.Results) == 0 {
		return output, pluginbinding.Fail("vision_failed", firstVisionError(output, "vision analysis returned no results"))
	}
	return output, nil
}

func (r Runner) visionProviders(ctx context.Context) []vision.Provider {
	var providers []vision.Provider
	for _, entry := range r.Marketplace.Plugins() {
		if entry.Name == visionPluginName || isBuiltinPlugin(entry) {
			continue
		}
		manifest, err := r.manifest(ctx, entry.Name)
		if err != nil {
			continue
		}
		provider, ok := vision.ProviderFromManifest(entry, manifest)
		if ok {
			providers = append(providers, provider)
		}
	}
	return providers
}

func (r Runner) runVision(ctx context.Context, instance string, input vision.AnalyzeInput) vision.AnalyzeOutput {
	if err := vision.ValidateImages(input.Images); err != nil {
		return vision.AnalyzeOutput{Errors: []vision.AnalyzeError{{Message: err.Error()}}}
	}
	providers, errors := vision.SelectProviders(r.visionProviders(ctx), input.Providers)
	output := vision.AnalyzeOutput{Errors: errors}
	if len(providers) == 0 {
		if len(output.Errors) == 0 {
			output.Errors = append(output.Errors, vision.AnalyzeError{Message: "no vision provider is available"})
		}
		return output
	}
	type jobResult struct {
		index  int
		result vision.AnalysisResult
		errors []vision.AnalyzeError
		err    error
	}
	results := make([]jobResult, len(providers))
	sem := make(chan struct{}, visionFanoutConcurrency)
	var wg sync.WaitGroup
	for i, provider := range providers {
		wg.Add(1)
		go func(index int, provider vision.Provider) {
			defer wg.Done()
			sem <- struct{}{}
			result, errors, err := r.runProviderVision(ctx, instance, provider, input)
			<-sem
			results[index] = jobResult{index: index, result: result, errors: errors, err: err}
		}(i, provider)
	}
	wg.Wait()
	for _, result := range results {
		provider := providers[result.index]
		for _, analysisErr := range result.errors {
			if strings.TrimSpace(analysisErr.Provider) == "" {
				analysisErr.Provider = provider.Name
			}
			output.Errors = append(output.Errors, analysisErr)
		}
		if result.err != nil {
			if len(result.errors) == 0 {
				output.Errors = append(output.Errors, vision.AnalyzeError{Provider: provider.Name, Message: result.err.Error()})
			}
			continue
		}
		if strings.TrimSpace(result.result.Text) == "" {
			output.Errors = append(output.Errors, vision.AnalyzeError{Provider: provider.Name, Message: "provider returned no analysis text"})
			continue
		}
		if strings.TrimSpace(result.result.Provider) == "" {
			result.result.Provider = provider.Name
		}
		output.Results = append(output.Results, result.result)
	}
	return output
}

func (r Runner) runProviderVision(ctx context.Context, instance string, provider vision.Provider, input vision.AnalyzeInput) (vision.AnalysisResult, []vision.AnalyzeError, error) {
	input.Model = vision.ModelForProvider(input, provider)
	inputRaw, err := json.Marshal(input)
	if err != nil {
		return vision.AnalysisResult{}, nil, err
	}
	resp, err := r.InvokeInstance(ctx, provider.Plugin, instance, protocol.CommandOperationsCall, protocol.OperationCall{Name: provider.Operation, Input: inputRaw})
	if err != nil {
		return vision.AnalysisResult{}, nil, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return vision.AnalysisResult{}, nil, fmt.Errorf("%s", resp.Error.Message)
		}
		return vision.AnalysisResult{}, nil, fmt.Errorf("provider operation failed")
	}
	var output vision.AnalyzeOutput
	if err := json.Unmarshal(resp.Result, &output); err != nil {
		return vision.AnalysisResult{}, nil, err
	}
	if len(output.Results) == 0 {
		return vision.AnalysisResult{}, output.Errors, fmt.Errorf("%s", firstVisionError(output, "provider returned no results"))
	}
	result := output.Results[0]
	if strings.TrimSpace(result.Provider) == "" {
		result.Provider = provider.Name
	}
	return result, output.Errors, nil
}

func firstVisionError(output vision.AnalyzeOutput, fallback string) string {
	if len(output.Errors) > 0 && strings.TrimSpace(output.Errors[0].Message) != "" {
		return output.Errors[0].Message
	}
	return fallback
}
