package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// TerminalPrompter implements dex.Prompter over an io.Reader/io.Writer pair.
// It reads sensitive values via the terminal in raw mode when stdin is a TTY.
type TerminalPrompter struct {
	In  io.Reader
	Out io.Writer

	bufReader *bufio.Reader
}

func (p *TerminalPrompter) reader() *bufio.Reader {
	if p.bufReader == nil {
		p.bufReader = bufio.NewReader(p.In)
	}
	return p.bufReader
}

func (p *TerminalPrompter) Confirm(_ context.Context, msg string) (bool, error) {
	_, _ = fmt.Fprintf(p.Out, "%s [y/N]: ", msg)
	line, err := p.reader().ReadString('\n')
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func (p *TerminalPrompter) Input(_ context.Context, label string) (string, error) {
	_, _ = fmt.Fprintf(p.Out, "%s: ", label)
	line, err := p.reader().ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (p *TerminalPrompter) Secret(_ context.Context, label string) (string, error) {
	_, _ = fmt.Fprintf(p.Out, "%s: ", label)
	if stdinIsTerminalReader(p.In) {
		data, err := term.ReadPassword(int(syscall.Stdin))
		_, _ = fmt.Fprintln(p.Out)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}
	line, err := p.reader().ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (p *TerminalPrompter) Print(_ context.Context, msg string) error {
	_, err := fmt.Fprintln(p.Out, msg)
	return err
}

func stdinIsTerminalReader(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}
