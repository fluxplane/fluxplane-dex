package sql

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

type Service struct{}

func NewService() Service {
	return Service{}
}

type QueryInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"required,description=Registered SQL endpoint ref resolved by the host."`
	Driver      string `json:"driver,omitempty" jsonschema:"description=SQL driver or dialect.,enum=mysql,enum=postgres,enum=sqlite"`
	Database    string `json:"database,omitempty" jsonschema:"description=Database override."`
	Query       string `json:"query,omitempty" jsonschema:"required,description=Read-only SQL query."`
	Timeout     string `json:"timeout,omitempty" jsonschema:"description=Query timeout as a Go duration such as 5s or 1m. Defaults to 10s."`
	MaxRows     int    `json:"max_rows,omitempty" jsonschema:"description=Maximum rows to return. Defaults to 100 and is capped at 1000.,minimum=0,maximum=1000"`
}

type QueryOutput struct {
	EndpointRef string           `json:"endpoint_ref,omitempty"`
	EndpointURL string           `json:"endpoint_url,omitempty"`
	Driver      string           `json:"driver,omitempty"`
	Database    string           `json:"database,omitempty"`
	Columns     []string         `json:"columns,omitempty"`
	Rows        []map[string]any `json:"rows,omitempty"`
	RowCount    int              `json:"row_count"`
	Truncated   bool             `json:"truncated,omitempty"`
	DurationMS  int64            `json:"duration_ms,omitempty"`
}

type QueryRowRecord struct {
	pluginbinding.DatasourceRecord
	RowID       string         `json:"row_id" datasource:"id,completion,view=compact|lookup|table"`
	Title       string         `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	Columns     []string       `json:"columns,omitempty" datasource:"view=table"`
	Row         map[string]any `json:"row,omitempty" datasource:"view=compact|lookup|table"`
	Driver      string         `json:"driver,omitempty" datasource:"completion,view=compact|lookup|table"`
	Database    string         `json:"database,omitempty" datasource:"completion,view=compact|lookup|table"`
	EndpointURL string         `json:"endpoint_url,omitempty" datasource:"completion,view=lookup|table"`
}

type QueryRowsResult = pluginbinding.DatasourceSearchResult[QueryRowRecord]

const readOnlySQLQueryMessage = "SQL query must be read-only; allowed statements are SELECT, SHOW, DESCRIBE, EXPLAIN, and WITH"

func (s Service) Query(ctx pluginbinding.Context, input QueryInput) (QueryOutput, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return QueryOutput{}, pluginbinding.Fail("bad_input", "query is required")
	}
	if !readOnlyQuery(query) {
		return QueryOutput{}, pluginbinding.Fail("bad_input", readOnlySQLQueryMessage)
	}
	if strings.TrimSpace(input.EndpointRef) == "" {
		return QueryOutput{}, pluginbinding.Fail("bad_input", "endpoint_ref is required")
	}
	if _, err := parseDurationDefault(input.Timeout, 10*time.Second); err != nil {
		return QueryOutput{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	if input.MaxRows < 0 {
		return QueryOutput{}, pluginbinding.Fail("bad_input", "max_rows must be non-negative")
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return QueryOutput{}, err
	}
	resp, err := ctx.Host.CapabilityCall(pluginbinding.ProviderCallRequest{
		Provider: PluginName,
		Action:   "query",
		Payload:  payload,
	})
	if err != nil {
		return QueryOutput{}, pluginbinding.Errorf("sql", "%s", err)
	}
	var out QueryOutput
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		return QueryOutput{}, pluginbinding.Errorf("sql", "decode host SQL result: %s", err)
	}
	return out, nil
}

func (s Service) QueryRows(ctx pluginbinding.Context, input QueryInput) (QueryRowsResult, error) {
	if strings.TrimSpace(input.Query) != "" && !readOnlyQuery(input.Query) {
		return QueryRowsResult{}, pluginbinding.Fail("bad_input", "SQL datasource search requires a read-only SQL query; pass SELECT, SHOW, DESCRIBE, EXPLAIN, or WITH SQL with --query")
	}
	out, err := s.Query(ctx, input)
	if err != nil {
		return QueryRowsResult{}, err
	}
	records := make([]QueryRowRecord, 0, len(out.Rows))
	for i, row := range out.Rows {
		rowID := sqlRowID(out, input.Query, i, row)
		title := sqlRowTitle(i, row, out.Columns)
		metadata := map[string]any{
			"columns":      out.Columns,
			"row":          row,
			"driver":       out.Driver,
			"database":     out.Database,
			"endpoint_url": out.EndpointURL,
			"endpoint_ref": out.EndpointRef,
			"query":        input.Query,
		}
		record := QueryRowRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(
				ctx.DatasourceSource(),
				EntitySQLQueryResult,
				rowID,
				pluginbinding.RecordTitle(title),
				pluginbinding.RecordMetadata(metadata),
			),
			RowID:       rowID,
			Title:       title,
			Columns:     append([]string(nil), out.Columns...),
			Row:         row,
			Driver:      out.Driver,
			Database:    out.Database,
			EndpointURL: out.EndpointURL,
		}
		if out.EndpointURL != "" {
			record.Links = map[string]string{"endpoint": out.EndpointURL}
		}
		records = append(records, record)
	}
	return pluginbinding.NewDatasourceSearchResult("live", input.Query, records), nil
}

func readOnlyQuery(query string) bool {
	trimmed := strings.TrimSpace(strings.TrimLeft(query, "("))
	if trimmed == "" {
		return false
	}
	tokens, hasStatementSeparator := sqlTokens(trimmed)
	if hasStatementSeparator || len(tokens) == 0 {
		return false
	}
	switch tokens[0] {
	case "select", "show", "describe", "desc", "explain", "with":
	default:
		return false
	}
	for i, token := range tokens {
		switch token {
		case "insert", "update", "delete", "drop", "create", "alter", "truncate", "replace", "grant", "revoke", "call", "do", "load", "copy", "execute", "merge":
			return false
		case "outfile", "dumpfile":
			if i > 0 && tokens[i-1] == "into" {
				return false
			}
		}
	}
	return true
}

func sqlTokens(query string) ([]string, bool) {
	tokens := []string{}
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		tokens = append(tokens, current.String())
		current.Reset()
	}
	for i := 0; i < len(query); i++ {
		ch := query[i]
		switch {
		case ch == ';':
			flush()
			return tokens, true
		case ch == '-' && i+1 < len(query) && query[i+1] == '-':
			flush()
			i += 2
			for i < len(query) && query[i] != '\n' && query[i] != '\r' {
				i++
			}
			i--
		case ch == '#':
			flush()
			for i < len(query) && query[i] != '\n' && query[i] != '\r' {
				i++
			}
			i--
		case ch == '/' && i+1 < len(query) && query[i+1] == '*':
			flush()
			i += 2
			for i+1 < len(query) && !(query[i] == '*' && query[i+1] == '/') {
				i++
			}
			i++
		case ch == '\'' || ch == '"' || ch == '`':
			flush()
			quote := ch
			for i++; i < len(query); i++ {
				if query[i] == '\\' && quote != '`' && i+1 < len(query) {
					i++
					continue
				}
				if query[i] != quote {
					continue
				}
				if quote == '\'' && i+1 < len(query) && query[i+1] == '\'' {
					i++
					continue
				}
				break
			}
		case ch == '_' || ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z':
			current.WriteByte(byte(strings.ToLower(string(ch))[0]))
		default:
			flush()
		}
	}
	flush()
	return tokens, false
}

func parseDurationDefault(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return time.ParseDuration(value)
}

func sqlRowID(out QueryOutput, query string, index int, row map[string]any) string {
	data, _ := json.Marshal(row)
	sum := sha1.Sum([]byte(out.Driver + "\x00" + out.Database + "\x00" + out.EndpointURL + "\x00" + query + "\x00" + strconv.Itoa(index) + "\x00" + string(data)))
	return hex.EncodeToString(sum[:])
}

func sqlRowTitle(index int, row map[string]any, columns []string) string {
	for _, column := range columns {
		if value, ok := row[column]; ok && value != nil {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" {
				return text
			}
		}
	}
	return fmt.Sprintf("row %d", index+1)
}
