package websearch

import "testing"

func TestNormalizeProvidersTreatsNamesAsOpaque(t *testing.T) {
	providers := NormalizeProviders([]string{" Tavily ", "ddg", "custom-provider", "ddg"})
	want := []string{"tavily", "ddg", "custom-provider"}
	if len(providers) != len(want) {
		t.Fatalf("providers = %#v", providers)
	}
	for i := range want {
		if providers[i] != want[i] {
			t.Fatalf("providers = %#v", providers)
		}
	}
}
