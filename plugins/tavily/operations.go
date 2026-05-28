package tavily

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/websearch"
)

const defaultEndpoint = "https://api.tavily.com/search"

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Service struct {
	SecretGetter pluginbinding.SecretGetter
	HTTPClient   HTTPDoer
	Endpoint     string
}

func NewService() Service {
	return Service{HTTPClient: http.DefaultClient, Endpoint: defaultEndpoint}
}

func (s Service) Search(ctx pluginbinding.Context, input websearch.SearchInput) (websearch.SearchOutput, error) {
	queries := websearch.NormalizeQueries(input)
	if len(queries) == 0 {
		return websearch.SearchOutput{}, pluginbinding.Fail("bad_input", "at least one query is required")
	}
	max := websearch.NormalizeMax(input)
	key, err := ctx.RequiredSecret(AuthPurposeAPIKey)
	if err != nil {
		return websearch.SearchOutput{}, err
	}
	output := websearch.SearchOutput{}
	for _, query := range queries {
		set, err := s.searchOne(context.Background(), strings.TrimSpace(key.Value), query, max)
		if err != nil {
			output.Errors = append(output.Errors, websearch.SearchError{Provider: PluginName, Query: query, Message: err.Error()})
			continue
		}
		output.Results = append(output.Results, set)
	}
	if len(output.Results) == 0 {
		return output, pluginbinding.Fail("web_search_failed", firstSearchError(output, "tavily search returned no results"))
	}
	return output, nil
}

func (s Service) searchOne(ctx context.Context, apiKey, query string, max int) (websearch.ResultSet, error) {
	body, err := json.Marshal(tavilySearchRequest{
		Query:             query,
		SearchDepth:       "basic",
		Topic:             "general",
		MaxResults:        max,
		IncludeAnswer:     false,
		IncludeRawContent: false,
		IncludeImages:     false,
	})
	if err != nil {
		return websearch.ResultSet{}, err
	}
	endpoint := strings.TrimSpace(s.Endpoint)
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return websearch.ResultSet{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "fluxplane-dex/0.1")
	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	httpClient := &http.Client{Timeout: 30 * time.Second}
	if client == http.DefaultClient {
		client = httpClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return websearch.ResultSet{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return websearch.ResultSet{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return websearch.ResultSet{}, fmt.Errorf("tavily search failed: %s: %s", resp.Status, tavilyErrorMessage(data))
	}
	var decoded tavilySearchResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return websearch.ResultSet{}, fmt.Errorf("decode tavily response: %w", err)
	}
	results := make([]websearch.Result, 0, len(decoded.Results))
	for _, result := range decoded.Results {
		url := strings.TrimSpace(result.URL)
		if url == "" {
			continue
		}
		results = append(results, websearch.Result{
			URL:     url,
			Title:   strings.TrimSpace(result.Title),
			Snippet: strings.TrimSpace(result.Content),
			Source:  PluginName,
			Score:   result.Score,
		})
	}
	return websearch.ResultSet{Provider: PluginName, Query: firstNonEmpty(decoded.Query, query), Answer: strings.TrimSpace(decoded.Answer), Results: results}, nil
}

type tavilySearchRequest struct {
	Query             string `json:"query"`
	SearchDepth       string `json:"search_depth"`
	Topic             string `json:"topic"`
	MaxResults        int    `json:"max_results"`
	IncludeAnswer     bool   `json:"include_answer"`
	IncludeRawContent bool   `json:"include_raw_content"`
	IncludeImages     bool   `json:"include_images"`
}

type tavilySearchResponse struct {
	Query   string               `json:"query"`
	Answer  string               `json:"answer"`
	Results []tavilySearchResult `json:"results"`
}

type tavilySearchResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

func tavilyErrorMessage(body []byte) string {
	var decoded struct {
		Detail any `json:"detail"`
	}
	if err := json.Unmarshal(body, &decoded); err == nil && decoded.Detail != nil {
		switch detail := decoded.Detail.(type) {
		case string:
			return detail
		case map[string]any:
			if msg, ok := detail["error"].(string); ok && strings.TrimSpace(msg) != "" {
				return msg
			}
		}
	}
	return strings.TrimSpace(string(body))
}

func firstSearchError(output websearch.SearchOutput, fallback string) string {
	if len(output.Errors) > 0 && strings.TrimSpace(output.Errors[0].Message) != "" {
		return output.Errors[0].Message
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
