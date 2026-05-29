package system

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const systemProvider = "system"

func Info(ctx pluginbinding.Context, input InfoInput) (InfoResult, error) {
	categories, err := selectCategories(input)
	if err != nil {
		return InfoResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	payload, err := json.Marshal(struct {
		Categories []string `json:"categories"`
	}{Categories: categories})
	if err != nil {
		return InfoResult{}, err
	}
	resp, err := ctx.Host.CapabilityCall(pluginbinding.ProviderCallRequest{
		Provider: systemProvider,
		Action:   "info",
		Payload:  payload,
	})
	if err != nil {
		return InfoResult{}, pluginbinding.Errorf("system", "%s", err)
	}
	var out InfoResult
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		return InfoResult{}, pluginbinding.Errorf("system", "decode host system info: %s", err)
	}
	if len(out.Categories) == 0 {
		out.Categories = categories
	}
	if out.System == nil {
		out.System = map[string]any{}
	}
	return out, nil
}

type StringList []string

func (s *StringList) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var values []string
	if err := json.Unmarshal(data, &values); err == nil {
		*s = splitSelectors(values...)
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("expected string or string array")
	}
	*s = splitSelectors(value)
	return nil
}

const (
	categoryOS      = "os"
	categoryRuntime = "runtime"
	categoryUser    = "user"
	categoryPaths   = "paths"
	categoryCPU     = "cpu"
	categoryTime    = "time"
	categoryEnv     = "env"
	categoryNetwork = "network"
)

var allCategories = []string{categoryOS, categoryRuntime, categoryUser, categoryPaths, categoryCPU, categoryTime, categoryEnv, categoryNetwork}

var categoryAliases = map[string]string{
	"arch":         categoryOS,
	"architecture": categoryOS,
	"cpus":         categoryCPU,
	"processor":    categoryCPU,
	"processors":   categoryCPU,
	"tmp":          categoryPaths,
	"temp":         categoryPaths,
	"tempdir":      categoryPaths,
	"timezone":     categoryTime,
}

func selectCategories(input InfoInput) ([]string, error) {
	requested := append([]string{}, input.Categories...)
	requested = append(requested, splitSelectors(input.Category)...)
	requested = append(requested, input.Include...)
	selected := map[string]bool{}
	if len(requested) == 0 {
		for _, category := range allCategories {
			selected[category] = true
		}
	} else {
		for _, value := range requested {
			category, err := normalizeCategory(value)
			if err != nil {
				return nil, err
			}
			selected[category] = true
		}
	}
	for _, value := range input.Exclude {
		category, err := normalizeCategory(value)
		if err != nil {
			return nil, err
		}
		delete(selected, category)
	}
	out := make([]string, 0, len(selected))
	for _, category := range allCategories {
		if selected[category] {
			out = append(out, category)
		}
	}
	return out, nil
}

func normalizeCategory(value string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(value))
	key = strings.ReplaceAll(key, "-", "_")
	if key == "" {
		return "", fmt.Errorf("empty category")
	}
	if alias, ok := categoryAliases[key]; ok {
		key = alias
	}
	for _, category := range allCategories {
		if key == category {
			return category, nil
		}
	}
	return "", fmt.Errorf("unknown category %q", value)
}

func splitSelectors(values ...string) []string {
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}
