package vision

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
)

func TestDefineProviderWiresManifestOperationAndSecrets(t *testing.T) {
	spec := ProviderSpec{
		Name:                 "example",
		Version:              "0.1.0",
		Description:          "Example vision provider.",
		Aliases:              []string{"ex"},
		Operation:            "example.vision.analyze",
		OperationDescription: "Analyze with Example.",
		Auth: []core.AuthMethod{pluginbinding.BearerAuth(
			"api_key",
			"Example API key.",
			pluginbinding.AuthField("api_key", "Example API key", true, true, "EXAMPLE_API_KEY"),
		)},
		SecretPurposes: []string{"api_key"},
	}
	plugin := DefineProvider(spec, func(_ pluginbinding.Context, _ AnalyzeInput) (AnalyzeOutput, error) {
		return AnalyzeOutput{Results: []AnalysisResult{{Provider: spec.Name, Text: "A chart."}}}, nil
	})
	manifest := plugin.Manifest()
	plugintest.AssertManifestQuality(t, manifest)
	if manifest.Metadata[MetadataProvider] != spec.Name || manifest.Metadata[MetadataOperation] != spec.Operation {
		t.Fatalf("metadata = %#v", manifest.Metadata)
	}
	if len(manifest.Operations) != 1 || manifest.Operations[0].Name != spec.Operation || manifest.Operations[0].SecretPurposes[0] != "api_key" {
		t.Fatalf("operations = %#v", manifest.Operations)
	}
	out := plugintest.RunOK[AnalyzeOutput](t, plugin, spec.Operation, AnalyzeInput{Images: []ImageInput{{URL: "https://example.com/a.png"}}})
	if len(out.Results) != 1 || out.Results[0].Text != "A chart." {
		t.Fatalf("output = %#v", out)
	}
}

func TestProviderDiscoveryFromManifestAndSelection(t *testing.T) {
	entry := core.PluginEntry{Name: "example"}
	manifest := pluginbinding.Manifest(ProviderManifestSpec(ProviderSpec{
		Name:      "example-provider",
		Aliases:   []string{"provider-alias"},
		Operation: "example.vision.analyze",
	}))
	provider, ok := ProviderFromManifest(entry, manifest)
	if !ok {
		t.Fatalf("provider not discovered")
	}
	if provider.Name != "example-provider" || provider.Plugin != "example" || provider.Operation != "example.vision.analyze" {
		t.Fatalf("provider = %#v", provider)
	}
	selected, errors := SelectProviders([]Provider{provider}, []string{"provider-alias"})
	if len(errors) != 0 || len(selected) != 1 || selected[0].Name != provider.Name {
		t.Fatalf("selected = %#v errors=%#v", selected, errors)
	}
	_, errors = SelectProviders([]Provider{provider}, []string{"missing"})
	if len(errors) != 1 || errors[0].Provider != "missing" {
		t.Fatalf("errors = %#v", errors)
	}
}

func TestDataURL(t *testing.T) {
	got, err := DataURL(ImageInput{URL: "https://example.com/a.webp"})
	if err != nil || got != "https://example.com/a.webp" {
		t.Fatalf("url = %q", got)
	}
	path := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(path, []byte("png bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = DataURL(ImageInput{FilePath: path})
	if err != nil || got != "data:image/png;base64,cG5nIGJ5dGVz" {
		t.Fatalf("file data url = %q err=%v", got, err)
	}
}

func TestModelForProviderPrefersProviderSpecificOverride(t *testing.T) {
	provider := Provider{Name: "openrouter", Plugin: "openrouter", Aliases: []string{"or"}}
	input := AnalyzeInput{
		Model: "fallback",
		Models: map[string]string{
			"or": "anthropic/claude-sonnet-latest",
		},
	}
	if got := ModelForProvider(input, provider); got != "anthropic/claude-sonnet-latest" {
		t.Fatalf("model = %q", got)
	}
}
