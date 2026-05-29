package vision

import (
	"encoding/base64"
	"mime"
	"path/filepath"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	DefaultPrompt = "Describe the image and extract any visible text."

	MetadataProvider  = "vision.provider"
	MetadataOperation = "vision.operation"
)

type AnalyzeInput struct {
	EndpointRef string            `json:"endpoint_ref,omitempty" jsonschema:"description=Registered provider endpoint ref resolved by the host."`
	Prompt      string            `json:"prompt,omitempty" jsonschema:"description=Text instruction for the image analysis. Defaults to a concise description and OCR request."`
	Images      []ImageInput      `json:"images,omitempty" jsonschema:"required,description=Images to analyze. Provide either url or base64 data for each image."`
	Providers   []string          `json:"providers,omitempty" jsonschema:"description=Optional provider names declared by vision provider plugins."`
	Model       string            `json:"model,omitempty" jsonschema:"description=Provider model override, for example gpt-4.1-mini or anthropic/claude-sonnet-latest."`
	Models      map[string]string `json:"models,omitempty" jsonschema:"description=Provider-specific model overrides keyed by provider name, plugin name, or alias."`
	MaxTokens   int               `json:"max_tokens,omitempty" jsonschema:"description=Provider max output tokens."`
	Temperature *float64          `json:"temperature,omitempty" jsonschema:"description=Provider sampling temperature."`
}

type ImageInput struct {
	URL          string `json:"url,omitempty" jsonschema:"description=Fully qualified image URL or data URL."`
	BlobRef      string `json:"blob_ref,omitempty" jsonschema:"description=Host-provided image blob reference."`
	ContentBytes []byte `json:"content_bytes,omitempty" jsonschema:"description=Base64-encoded inline image bytes."`
	Filename     string `json:"filename,omitempty" jsonschema:"description=Filename hint for media type detection."`
	MediaType    string `json:"media_type,omitempty" jsonschema:"description=Media type for raw base64 data, for example image/jpeg.,enum=image/jpeg,enum=image/png,enum=image/webp,enum=image/gif"`
	Detail       string `json:"detail,omitempty" jsonschema:"description=Optional provider detail hint.,enum=auto,enum=low,enum=high"`
}

type AnalyzeOutput struct {
	Results []AnalysisResult `json:"results,omitempty"`
	Errors  []AnalyzeError   `json:"errors,omitempty"`
}

type AnalysisResult struct {
	Provider string         `json:"provider"`
	Model    string         `json:"model,omitempty"`
	Text     string         `json:"text"`
	Usage    map[string]any `json:"usage,omitempty"`
}

type AnalyzeError struct {
	Provider string `json:"provider,omitempty"`
	Message  string `json:"message"`
}

type Provider struct {
	Name      string   `json:"name"`
	Plugin    string   `json:"plugin"`
	Aliases   []string `json:"aliases,omitempty"`
	Operation string   `json:"operation"`
}

type ProviderListResult struct {
	Providers []Provider `json:"providers"`
	Count     int        `json:"count"`
}

type ProviderSpec struct {
	Name                 string
	Version              string
	Description          string
	Aliases              []string
	Operation            string
	OperationDescription string
	Auth                 []core.AuthMethod
	SecretPurposes       []string
}

type AnalyzeHandler func(pluginbinding.Context, AnalyzeInput) (AnalyzeOutput, error)

func ProviderManifestSpec(spec ProviderSpec) pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        spec.Name,
		Version:     spec.Version,
		Description: spec.Description,
		Aliases:     append([]string(nil), spec.Aliases...),
		Auth:        append([]core.AuthMethod(nil), spec.Auth...),
		Operations: []core.OperationSpec{
			ProviderOperationSpec(spec),
		},
		Metadata: ProviderMetadata(spec),
	}
}

func ProviderMetadata(spec ProviderSpec) map[string]string {
	return map[string]string{
		MetadataProvider:  spec.Name,
		MetadataOperation: spec.Operation,
	}
}

func DefineProvider(spec ProviderSpec, analyze AnalyzeHandler, options ...pluginbinding.PluginOption) *pluginbinding.Plugin {
	bindings := []pluginbinding.PluginOption{
		pluginbinding.RegisterOperation(ProviderOperationSpec(spec), pluginbinding.OperationHandler[AnalyzeInput, AnalyzeOutput](analyze)),
	}
	return pluginbinding.Define(ProviderManifestSpec(spec), append(options, bindings...)...)
}

func ProviderOperationSpec(spec ProviderSpec) core.OperationSpec {
	options := []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.Compact(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork, core.OperationEffectFilesystem),
		pluginbinding.Access(core.OperationAccessNetwork, core.OperationAccessFilesystem),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	}
	if len(spec.SecretPurposes) > 0 {
		options = append(options, pluginbinding.SecretPurposes(spec.SecretPurposes...))
	}
	return pluginbinding.TypedOperationSpec[AnalyzeInput, AnalyzeOutput](
		spec.Operation,
		firstNonEmpty(spec.OperationDescription, "Analyze images with "+spec.Name+"."),
		options...,
	)
}

func ProviderFromManifest(entry core.PluginEntry, manifest core.PluginManifest) (Provider, bool) {
	providerName := strings.TrimSpace(manifest.Metadata[MetadataProvider])
	operation := strings.TrimSpace(manifest.Metadata[MetadataOperation])
	if providerName == "" || operation == "" {
		return Provider{}, false
	}
	return Provider{
		Name:      providerName,
		Plugin:    entry.Name,
		Aliases:   uniqueStrings(append([]string(nil), manifest.Aliases...)),
		Operation: operation,
	}, true
}

func SelectProviders(available []Provider, requested []string) ([]Provider, []AnalyzeError) {
	if len(requested) == 0 {
		return available, nil
	}
	var selected []Provider
	var errors []AnalyzeError
	seen := map[string]bool{}
	for _, raw := range requested {
		name := NormalizeProviderName(raw)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		provider, ok := FindProvider(available, name)
		if !ok {
			errors = append(errors, AnalyzeError{Provider: raw, Message: "vision provider not found"})
			continue
		}
		selected = append(selected, provider)
	}
	return selected, errors
}

func FindProvider(providers []Provider, name string) (Provider, bool) {
	name = NormalizeProviderName(name)
	for _, provider := range providers {
		if NormalizeProviderName(provider.Name) == name || NormalizeProviderName(provider.Plugin) == name {
			return provider, true
		}
		for _, alias := range provider.Aliases {
			if NormalizeProviderName(alias) == name {
				return provider, true
			}
		}
	}
	return Provider{}, false
}

func NormalizeProviderName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func NormalizePrompt(value string) string {
	if prompt := strings.TrimSpace(value); prompt != "" {
		return prompt
	}
	return DefaultPrompt
}

func ModelForProvider(input AnalyzeInput, provider Provider) string {
	for _, key := range append([]string{provider.Name, provider.Plugin}, provider.Aliases...) {
		if model := strings.TrimSpace(input.Models[key]); model != "" {
			return model
		}
		if model := strings.TrimSpace(input.Models[NormalizeProviderName(key)]); model != "" {
			return model
		}
	}
	return strings.TrimSpace(input.Model)
}

func ValidateImages(images []ImageInput) error {
	if len(images) == 0 {
		return pluginbinding.Fail("bad_input", "at least one image is required")
	}
	for i, image := range images {
		if strings.TrimSpace(image.URL) == "" && strings.TrimSpace(image.BlobRef) == "" && len(image.ContentBytes) == 0 {
			return pluginbinding.Errorf("bad_input", "image %d requires url, blob_ref, or content_bytes", i)
		}
	}
	return nil
}

func DataURL(image ImageInput) (string, error) {
	if len(image.ContentBytes) > 0 {
		return DataURLFromBytes(image.ContentBytes, image.MediaType, image.Filename), nil
	}
	return strings.TrimSpace(image.URL), nil
}

func DataURLFromBytes(data []byte, mediaType, filename string) string {
	mediaType = NormalizeMediaType(mediaType, filename, data)
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func NormalizeMediaType(mediaType, path string, data []byte) string {
	if mediaType = strings.TrimSpace(mediaType); mediaType != "" {
		return mediaType
	}
	if detected := mediaTypeByExtension(path); detected != "" {
		return detected
	}
	if detected := detectMediaType(data); detected != "" {
		return detected
	}
	return "image/jpeg"
}

func mediaTypeByExtension(path string) string {
	if ext := strings.TrimSpace(filepath.Ext(path)); ext != "" {
		if detected := mime.TypeByExtension(ext); strings.HasPrefix(detected, "image/") {
			return detected
		}
	}
	return ""
}

func detectMediaType(data []byte) string {
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "image/gif"
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}
