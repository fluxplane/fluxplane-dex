package yaml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func Marshal(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var decoded any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		return nil, err
	}
	var b strings.Builder
	writeValue(&b, decoded, 0)
	return []byte(b.String()), nil
}

func writeValue(b *strings.Builder, v any, indent int) {
	pad := strings.Repeat("  ", indent)
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for key := range x {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			b.WriteString(pad)
			b.WriteString(key)
			if scalar(x[key]) {
				b.WriteString(": ")
				writeScalar(b, x[key])
				b.WriteByte('\n')
			} else {
				b.WriteString(":\n")
				writeValue(b, x[key], indent+1)
			}
		}
	case []any:
		for _, item := range x {
			b.WriteString(pad)
			if scalar(item) {
				b.WriteString("- ")
				writeScalar(b, item)
				b.WriteByte('\n')
			} else {
				b.WriteString("-\n")
				writeValue(b, item, indent+1)
			}
		}
	default:
		b.WriteString(pad)
		writeScalar(b, x)
		b.WriteByte('\n')
	}
}

func scalar(v any) bool {
	switch v.(type) {
	case nil, string, bool, json.Number, float64:
		return true
	default:
		return false
	}
}

func writeScalar(b *strings.Builder, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case string:
		if x == "" || strings.ContainsAny(x, ":\n#[]{}") {
			encoded, _ := json.Marshal(x)
			b.Write(encoded)
			return
		}
		b.WriteString(x)
	case json.Number:
		b.WriteString(x.String())
	default:
		b.WriteString(fmt.Sprint(x))
	}
}
