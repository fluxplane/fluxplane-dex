package dex

import (
	"errors"
	"fmt"

	"github.com/fluxplane/fluxplane-dex/protocol"
)

// Sentinel errors. Callers can use errors.Is to detect these.
var (
	ErrPluginNotFound     = errors.New("dex: plugin not found in marketplace")
	ErrPluginNotInstalled = errors.New("dex: plugin not installed")
	ErrAuthRequired       = errors.New("dex: auth required")
	ErrInstanceUnknown    = errors.New("dex: unknown instance")
	ErrNoPrompter         = errors.New("dex: no interactive prompter configured")
	ErrMissingFields      = errors.New("dex: required fields missing")
)

// PluginError wraps a *protocol.Error returned from a plugin so the
// underlying code/message stays accessible alongside the Go error string.
type PluginError struct {
	Plugin string
	Cause  *protocol.Error
}

func (e *PluginError) Error() string {
	if e == nil || e.Cause == nil {
		return "dex: plugin error"
	}
	if e.Cause.Code != "" {
		return fmt.Sprintf("dex: %s: %s (%s)", e.Plugin, e.Cause.Message, e.Cause.Code)
	}
	return fmt.Sprintf("dex: %s: %s", e.Plugin, e.Cause.Message)
}

// asPluginError returns a typed PluginError if resp carries one; otherwise nil.
func asPluginError(plugin string, resp protocol.Response) error {
	if resp.OK || resp.Error == nil {
		return nil
	}
	return &PluginError{Plugin: plugin, Cause: resp.Error}
}
