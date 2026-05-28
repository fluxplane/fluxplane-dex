package system

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

func BuildContext(_ pluginbinding.Context, input pluginbinding.ContextBuildInput) (pluginbinding.ContextBuildResult, error) {
	now := time.Now()
	categories := []string{categoryOS, categoryRuntime, categoryUser, categoryPaths, categoryCPU, categoryTime, categoryNetwork}
	summary := []string{
		fmt.Sprintf("OS: %s/%s", runtime.GOOS, runtime.GOARCH),
		fmt.Sprintf("CPUs: %d logical, GOMAXPROCS=%d", runtime.NumCPU(), runtime.GOMAXPROCS(0)),
		fmt.Sprintf("Time: %s", now.Format(time.RFC3339)),
		fmt.Sprintf("Categories available: %s", strings.Join(categories, ", ")),
	}
	query := strings.TrimSpace(input.Query)
	if query != "" {
		summary = append(summary, "Query: "+query)
	}
	return pluginbinding.ContextBuildResult{
		Blocks: []core.ContextBlock{{
			ID:       ContextName,
			Kind:     pluginbinding.ContextKindText,
			Title:    "Local system context",
			Content:  strings.Join(summary, "\n"),
			Priority: 50,
			Metadata: map[string]string{
				"categories": strings.Join(categories, ","),
			},
		}},
	}, nil
}
