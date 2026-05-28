package dex

import "context"

// Prompter abstracts interactive user input. Operations that need to
// confirm an action, read a value, or read a secret call methods on the
// engine's Prompter. The CLI installs a terminal-backed prompter; embedders
// like fluxplane-core can install one that drives the user via Slack, the
// web UI, etc.
type Prompter interface {
	Confirm(ctx context.Context, msg string) (bool, error)
	Input(ctx context.Context, label string) (string, error)
	Secret(ctx context.Context, label string) (string, error)
	Print(ctx context.Context, msg string) error
}

// NoopPrompter is the default Prompter used when none is configured. All
// methods return ErrNoPrompter so callers can detect the missing capability
// and react (e.g. fall back to env-var-based auto-connect).
type NoopPrompter struct{}

func (NoopPrompter) Confirm(context.Context, string) (bool, error)   { return false, ErrNoPrompter }
func (NoopPrompter) Input(context.Context, string) (string, error)   { return "", ErrNoPrompter }
func (NoopPrompter) Secret(context.Context, string) (string, error)  { return "", ErrNoPrompter }
func (NoopPrompter) Print(context.Context, string) error             { return nil }
