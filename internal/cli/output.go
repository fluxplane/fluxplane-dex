package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/fluxplane/fluxplane-dex/internal/yaml"
)

func render(out io.Writer, format string, raw json.RawMessage) error {
	var value any
	if len(raw) == 0 {
		value = map[string]any{}
	} else if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	switch format {
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	case "yaml":
		data, err := yaml.Marshal(value)
		if err != nil {
			return err
		}
		_, err = out.Write(data)
		return err
	case "compact":
		if text, ok := stringField(value, "summary"); ok {
			_, err := fmt.Fprintln(out, text)
			return err
		}
		return renderOneLineJSON(out, value)
	case "text", "":
		if text, ok := stringField(value, "text"); ok {
			_, err := fmt.Fprintln(out, text)
			return err
		}
		return renderOneLineJSON(out, value)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func renderValue(out io.Writer, format string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return render(out, format, data)
}

func renderOneLineJSON(out io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}

func stringField(value any, key string) (string, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	text, ok := m[key].(string)
	return strings.TrimSpace(text), ok && strings.TrimSpace(text) != ""
}
