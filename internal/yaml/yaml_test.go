package yaml

import (
	"strings"
	"testing"
)

func TestMarshalPreservesIntegerText(t *testing.T) {
	data, err := Marshal(map[string]any{
		"small": 25,
		"large": int64(123456789012345678),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{"small: 25", "large: 123456789012345678"} {
		if !strings.Contains(got, want) {
			t.Fatalf("yaml missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "e+") || strings.Contains(got, "E+") {
		t.Fatalf("integer rendered with exponent:\n%s", got)
	}
}
