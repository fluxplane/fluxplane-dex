package dex

import (
	"context"

	"github.com/fluxplane/fluxplane-dex/runtime"
)

// PluginEvent mirrors runtime.PluginEvent — a best-effort, plugin-emitted
// progress/status payload.
type PluginEvent = runtime.PluginEvent

// EventSink receives plugin-emitted events. Implementations should not block;
// the event loop calls the sink synchronously.
type EventSink func(context.Context, PluginEvent)
