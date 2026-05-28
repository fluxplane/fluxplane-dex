package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/fluxplane/fluxplane-dex/internal/yaml"
)

func render(out io.Writer, format string, raw json.RawMessage) error {
	if len(raw) == 0 {
		return renderDecoded(out, format, map[string]any{})
	}
	var value any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return err
	}
	return renderDecoded(out, format, value)
}

func renderValue(out io.Writer, format string, value any) error {
	return renderDecoded(out, format, value)
}

func renderDecoded(out io.Writer, format string, value any) error {
	value, err := normalizeOutputValue(value)
	if err != nil {
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
		_, err = fmt.Fprintln(out, compactText(value))
		return err
	case "text", "":
		_, err = fmt.Fprint(out, textOutput(value))
		return err
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func normalizeOutputValue(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var decoded any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func textOutput(value any) string {
	if message, ok := emptyResultMessage(value); ok {
		return message + "\n"
	}
	if text, ok := stringField(value, "text"); ok {
		return text + "\n"
	}
	if summary, ok := stringField(value, "summary"); ok {
		return summary + "\n"
	}
	var b strings.Builder
	writeTextValue(&b, value, 0)
	return b.String()
}

func compactText(value any) string {
	if message, ok := emptyResultMessage(value); ok {
		return message
	}
	if summary, ok := stringField(value, "summary"); ok {
		return summary
	}
	if text, ok := stringField(value, "text"); ok {
		return text
	}
	if count, label, ok := collectionSummary(value); ok {
		return fmt.Sprintf("%d %s", count, label)
	}
	return compactValue(value)
}

func writeTextValue(b *strings.Builder, value any, indent int) {
	pad := strings.Repeat("  ", indent)
	switch x := value.(type) {
	case map[string]any:
		keys := sortedMapKeys(x)
		for _, key := range keys {
			item := x[key]
			if scalar(item) {
				b.WriteString(pad)
				b.WriteString(key)
				b.WriteString(": ")
				b.WriteString(scalarString(item))
				b.WriteByte('\n')
				continue
			}
			b.WriteString(pad)
			b.WriteString(key)
			b.WriteString(":\n")
			writeTextValue(b, item, indent+1)
		}
	case []any:
		for _, item := range x {
			b.WriteString(pad)
			if scalar(item) {
				b.WriteString("- ")
				b.WriteString(scalarString(item))
				b.WriteByte('\n')
				continue
			}
			b.WriteString("-\n")
			writeTextValue(b, item, indent+1)
		}
	default:
		b.WriteString(pad)
		b.WriteString(scalarString(x))
		b.WriteByte('\n')
	}
}

func compactValue(value any) string {
	switch x := value.(type) {
	case map[string]any:
		keys := sortedMapKeys(x)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			item := x[key]
			if scalar(item) {
				parts = append(parts, key+"="+scalarString(item))
				continue
			}
			if n, ok := collectionLen(item); ok {
				parts = append(parts, fmt.Sprintf("%s=%d", key, n))
				continue
			}
			parts = append(parts, key+"="+oneLineJSON(item))
		}
		return strings.Join(parts, " ")
	case []any:
		parts := make([]string, 0, len(x))
		for _, item := range x {
			parts = append(parts, compactValue(item))
		}
		return strings.Join(parts, ", ")
	default:
		return scalarString(x)
	}
}

func oneLineJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

func emptyResultMessage(value any) (string, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	key, ok := emptyResultKey(m)
	if !ok {
		return "", false
	}
	if key == "matches" {
		if text, ok := stringField(value, "text"); ok {
			return fmt.Sprintf("no matches for %q", text), true
		}
		return "no matches", true
	}
	if text, ok := stringField(value, "text"); ok {
		return fmt.Sprintf("no matches for %q", text), true
	}
	if query, ok := stringField(value, "query"); ok {
		return fmt.Sprintf("no results for %q", query), true
	}
	return "no results", true
}

func emptyResultKey(m map[string]any) (string, bool) {
	for _, key := range collectionKeys() {
		value, ok := m[key]
		if !ok || !emptyCollection(value) {
			continue
		}
		return key, true
	}
	return "", false
}

func collectionSummary(value any) (int, string, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return 0, "", false
	}
	for _, key := range collectionKeys() {
		if n, ok := collectionLen(m[key]); ok {
			return n, key, true
		}
	}
	return 0, "", false
}

func collectionKeys() []string {
	return append([]string{"results"}, recordSourceKeys()...)
}

func recordSourceKeys() []string {
	return []string{
		"records",
		"matches",
		"items",
		"plugins",
		"datasources",
		"operations",
		"endpoints",
		"contexts",
		"namespaces",
		"pods",
		"services",
		"deployments",
		"containers",
		"blocks",
		"candidates",
		"indexes",
	}
}

func emptyCollection(value any) bool {
	n, ok := collectionLen(value)
	return ok && n == 0
}

func collectionLen(value any) (int, bool) {
	switch x := value.(type) {
	case []any:
		return len(x), true
	case map[string]any:
		if fanoutEnvelope(x) {
			return fanoutAvailableLen(x), true
		}
		return len(x), true
	default:
		return 0, false
	}
}

func fanoutEnvelope(m map[string]any) bool {
	_, hasAvailable := m["available"]
	_, hasMissing := m["missing"]
	_, hasErrors := m["errors"]
	return hasAvailable && !hasMissing && !hasErrors
}

func fanoutAvailableLen(m map[string]any) int {
	available, _ := collectionLenNoEnvelope(m["available"])
	return available
}

func collectionLenNoEnvelope(value any) (int, bool) {
	switch x := value.(type) {
	case []any:
		return len(x), true
	case map[string]any:
		return len(x), true
	default:
		return 0, false
	}
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func scalar(value any) bool {
	switch value.(type) {
	case nil, string, bool, json.Number, float64, float32,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}

func scalarString(value any) string {
	switch x := value.(type) {
	case nil:
		return "null"
	case string:
		return x
	case json.Number:
		return x.String()
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}

func stringField(value any, key string) (string, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	text, ok := m[key].(string)
	return strings.TrimSpace(text), ok && strings.TrimSpace(text) != ""
}
