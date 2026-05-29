package duckduckgo

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/websearch"
)

const defaultEndpointTemplate = "https://html.duckduckgo.com/html/?q={query}"

type Service struct {
	EndpointTemplate string
}

func NewService() Service {
	return Service{EndpointTemplate: defaultEndpointTemplate}
}

func (s Service) Search(ctx pluginbinding.Context, input websearch.SearchInput) (websearch.SearchOutput, error) {
	queries, err := websearch.ValidateQueries(input)
	if err != nil {
		return websearch.SearchOutput{}, err
	}
	max := websearch.NormalizeMax(input)
	output := websearch.SearchOutput{}
	for _, query := range queries {
		set, err := s.searchOne(ctx, query, max)
		if err != nil {
			output.Errors = append(output.Errors, websearch.SearchError{Provider: PluginName, Query: query, Message: err.Error()})
			continue
		}
		if !websearch.HasResults(set) {
			output.Errors = append(output.Errors, websearch.SearchError{Provider: PluginName, Query: query, Message: "duckduckgo search returned no results"})
			continue
		}
		output.Results = append(output.Results, set)
	}
	if len(output.Results) == 0 {
		return output, pluginbinding.Fail("web_search_failed", firstSearchError(output, "duckduckgo search returned no results"))
	}
	return output, nil
}

func (s Service) searchOne(ctx pluginbinding.Context, query string, max int) (websearch.ResultSet, error) {
	template := strings.TrimSpace(s.EndpointTemplate)
	if template == "" {
		template = defaultEndpointTemplate
	}
	resp, err := ctx.Host.HTTP(pluginbinding.HTTPRequest{
		URL:       searchURL(template, query),
		Method:    "GET",
		UserAgent: "fluxplane-dex/0.1",
		TimeoutMS: 30000,
		MaxBytes:  512 * 1024,
	})
	if err != nil {
		return websearch.ResultSet{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return websearch.ResultSet{}, fmt.Errorf("duckduckgo search failed: %s", responseStatus(resp))
	}
	return websearch.ResultSet{Provider: PluginName, Query: query, Results: parseResults(string(resp.Body), max)}, nil
}

func searchURL(template, query string) string {
	escaped := queryEscape(query)
	if strings.Contains(template, "{query}") {
		return strings.ReplaceAll(template, "{query}", escaped)
	}
	separator := "?"
	if strings.Contains(template, "?") {
		separator = "&"
	}
	return template + separator + "q=" + escaped
}

func queryEscape(value string) string {
	const hex = "0123456789ABCDEF"
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.', c == '~':
			out.WriteByte(c)
		case c == ' ':
			out.WriteByte('+')
		default:
			out.WriteByte('%')
			out.WriteByte(hex[c>>4])
			out.WriteByte(hex[c&0x0f])
		}
	}
	return out.String()
}

var (
	resultLinkRE = regexp.MustCompile(`(?is)<a[^>]+class=["'][^"']*result__a[^"']*["'][^>]+href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	snippetRE    = regexp.MustCompile(`(?is)<(?:a|div)[^>]+class=["'][^"']*result__snippet[^"']*["'][^>]*>(.*?)</(?:a|div)>`)
	tagRE        = regexp.MustCompile(`(?is)<[^>]+>`)
)

func parseResults(body string, limit int) []websearch.Result {
	matches := resultLinkRE.FindAllStringSubmatchIndex(body, -1)
	results := make([]websearch.Result, 0, minInt(len(matches), limit))
	for i, match := range matches {
		if limit > 0 && len(results) >= limit {
			break
		}
		url := normalizeResultURL(body[match[2]:match[3]])
		title := cleanHTML(body[match[4]:match[5]])
		if url == "" || title == "" {
			continue
		}
		nextStart := len(body)
		if i+1 < len(matches) {
			nextStart = matches[i+1][0]
		}
		window := body[match[1]:nextStart]
		snippet := ""
		if snippetMatch := snippetRE.FindStringSubmatch(window); len(snippetMatch) > 1 {
			snippet = cleanHTML(snippetMatch[1])
		}
		results = append(results, websearch.Result{URL: url, Title: title, Snippet: snippet, Source: PluginName})
	}
	return results
}

func normalizeResultURL(raw string) string {
	value := html.UnescapeString(strings.TrimSpace(raw))
	if strings.HasPrefix(value, "//") {
		value = "https:" + value
	}
	if idx := strings.Index(value, "uddg="); idx >= 0 {
		encoded := value[idx+len("uddg="):]
		if end := strings.IndexByte(encoded, '&'); end >= 0 {
			encoded = encoded[:end]
		}
		if decoded := percentDecode(encoded); decoded != "" {
			value = decoded
		}
	}
	return websearch.NormalizeResultURL(value)
}

func percentDecode(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '%' && i+2 < len(value) {
			hi, okHi := hexValue(value[i+1])
			lo, okLo := hexValue(value[i+2])
			if okHi && okLo {
				out.WriteByte(hi<<4 | lo)
				i += 2
				continue
			}
		}
		if value[i] == '+' {
			out.WriteByte(' ')
			continue
		}
		out.WriteByte(value[i])
	}
	return out.String()
}

func hexValue(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

func cleanHTML(value string) string {
	text := tagRE.ReplaceAllString(value, " ")
	text = html.UnescapeString(text)
	return strings.Join(strings.Fields(text), " ")
}

func firstSearchError(output websearch.SearchOutput, fallback string) string {
	if len(output.Errors) > 0 && strings.TrimSpace(output.Errors[0].Message) != "" {
		return output.Errors[0].Message
	}
	return fallback
}

func responseStatus(resp pluginbinding.HTTPResponse) string {
	if strings.TrimSpace(resp.Status) != "" {
		return strings.TrimSpace(resp.Status)
	}
	if resp.StatusCode != 0 {
		return fmt.Sprintf("%d", resp.StatusCode)
	}
	return "unknown status"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
