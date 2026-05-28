package runtime

import "testing"

func TestLoadMarketplaceResolvesCanonicalNames(t *testing.T) {
	marketplace, err := LoadMarketplaceData([]byte(`{"version":"1","plugins":[{"name":"example","aliases":["ex"],"binary":"dex-plugin-example","go_install":"example.com/plugin@latest"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	plugin, ok := marketplace.Resolve("example")
	if !ok {
		t.Fatal("plugin did not resolve")
	}
	if plugin.Name != "example" {
		t.Fatalf("plugin resolved to %q", plugin.Name)
	}
	if plugin.Binary == "" || plugin.GoInstall == "" {
		t.Fatalf("plugin install metadata incomplete: %#v", plugin)
	}
	if _, ok := marketplace.Resolve("ex"); ok {
		t.Fatal("marketplace aliases should not resolve")
	}
}

func TestLoadMarketplaceListsPlugins(t *testing.T) {
	marketplace, err := LoadMarketplaceData([]byte(`{"version":"1","plugins":[{"name":"one","binary":"dex-plugin-one"},{"name":"two","binary":"dex-plugin-two"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	plugins := marketplace.Plugins()
	if len(plugins) != 2 {
		t.Fatalf("plugins = %d, want 2", len(plugins))
	}
}
