package runtime

import (
	"os"
	"path/filepath"
	"strings"
)

type State struct {
	Home string
}

func NewState(home string) (State, error) {
	if strings.TrimSpace(home) == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return State{}, err
		}
		home = filepath.Join(userHome, ".dex")
	}
	home = expandHome(home)
	for _, dir := range []string{home, filepath.Join(home, "auth"), filepath.Join(home, "plugins"), filepath.Join(home, "grants"), filepath.Join(home, "indexes")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return State{}, err
		}
	}
	return State{Home: home}, nil
}

func (s State) AuthDir() string {
	return filepath.Join(s.Home, "auth")
}

func (s State) StateDBPath() string {
	return filepath.Join(s.Home, "state.db")
}

func expandHome(path string) string {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
