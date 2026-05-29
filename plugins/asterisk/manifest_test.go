package asterisk

import "testing"

func TestManifestDeclaresAMIPingAndEndpointDiscovery(t *testing.T) {
	manifest := Manifest()
	if manifest.Name != PluginName {
		t.Fatalf("name = %q", manifest.Name)
	}
	if len(manifest.Operations) != 1 || manifest.Operations[0].Name != OperationAMIPing {
		t.Fatalf("operations = %#v", manifest.Operations)
	}
	if len(manifest.Endpoints) != 1 {
		t.Fatalf("endpoints = %#v", manifest.Endpoints)
	}
	if !containsString(manifest.Endpoints[0].Products, "asterisk") || !containsString(manifest.Endpoints[0].Products, "ami") {
		t.Fatalf("endpoint products = %#v", manifest.Endpoints[0].Products)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
