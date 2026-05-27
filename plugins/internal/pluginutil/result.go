package pluginutil

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fluxplane/fluxplane-dex/protocol"
)

type Result struct {
	Text    string `json:"text,omitempty"`
	Summary string `json:"summary,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func OKText(text string, data any) protocol.Response {
	return protocol.OK(Result{Text: text, Summary: firstLine(text), Data: data})
}

func OKData(data any) protocol.Response {
	return protocol.OK(Result{Data: data})
}

func DecodeInput(req protocol.Request) (map[string]any, error) {
	call, err := protocol.DecodePayload[protocol.OperationCall](req.Payload)
	if err != nil {
		return nil, err
	}
	var input map[string]any
	if len(call.Input) > 0 {
		if err := json.Unmarshal(call.Input, &input); err != nil {
			return nil, fmt.Errorf("decode operation input: %w", err)
		}
	}
	if input == nil {
		input = map[string]any{}
	}
	return input, nil
}

func String(input map[string]any, key string) string {
	value, _ := input[key].(string)
	return strings.TrimSpace(value)
}

func ReadJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

type SecretMaterial struct {
	Kind    string `json:"kind,omitempty"`
	Value   string `json:"value"`
	Source  string `json:"source,omitempty"`
	Purpose string `json:"purpose,omitempty"`
}

func SecretGet(plugin, instance, grant, purpose string) (SecretMaterial, error) {
	host := strings.TrimSpace(os.Getenv("DEX_HOST_CMD"))
	if host == "" {
		host = "dex"
	}
	args := []string{"secret", "get", plugin, "--instance", instance, "--grant", grant, "--purpose", purpose, "-o", "json"}
	cmd := exec.Command(host, args...)
	data, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && len(exit.Stderr) > 0 {
			return SecretMaterial{}, fmt.Errorf("%s", strings.TrimSpace(string(exit.Stderr)))
		}
		return SecretMaterial{}, err
	}
	var material SecretMaterial
	if err := json.Unmarshal(data, &material); err != nil {
		return SecretMaterial{}, err
	}
	return material, nil
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}
