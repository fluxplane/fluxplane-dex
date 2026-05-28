package pluginbinding

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type SecretMaterial struct {
	Kind    string `json:"kind,omitempty"`
	Value   string `json:"value"`
	Source  string `json:"source,omitempty"`
	Purpose string `json:"purpose,omitempty"`
}

type SecretGetter func(Context, string) (SecretMaterial, error)

const secretCachePrefix = "pluginbinding.secret."

func (ctx Context) Secret(purpose string) (SecretMaterial, error) {
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		return SecretMaterial{}, fmt.Errorf("secret purpose is empty")
	}
	if ctx.Cache != nil {
		if value, ok := ctx.Cache.Get(secretCachePrefix + purpose); ok {
			if material, ok := value.(SecretMaterial); ok {
				return material, nil
			}
		}
	}
	getter := DefaultSecretGetter
	if ctx.plugin != nil && ctx.plugin.secretGetter != nil {
		getter = ctx.plugin.secretGetter
	}
	material, err := getter(ctx, purpose)
	if err != nil {
		return SecretMaterial{}, Errorf("secret", "%s", err)
	}
	if material.Purpose == "" {
		material.Purpose = purpose
	}
	if ctx.Cache != nil {
		ctx.Cache.Set(secretCachePrefix+purpose, material)
	}
	return material, nil
}

func (ctx Context) OptionalSecret(purpose string) (SecretMaterial, bool) {
	material, err := ctx.Secret(purpose)
	if err != nil || strings.TrimSpace(material.Value) == "" {
		return SecretMaterial{}, false
	}
	return material, true
}

func (ctx Context) RequiredSecrets(purposes ...string) (map[string]SecretMaterial, error) {
	out := make(map[string]SecretMaterial, len(purposes))
	for _, purpose := range purposes {
		material, err := ctx.Secret(purpose)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(material.Value) == "" {
			return nil, Fail("secret", purpose+" is empty")
		}
		out[purpose] = material
	}
	return out, nil
}

func (ctx Context) RequiredSecret(purpose string) (SecretMaterial, error) {
	material, err := ctx.Secret(purpose)
	if err != nil {
		return SecretMaterial{}, err
	}
	if strings.TrimSpace(material.Value) == "" {
		return SecretMaterial{}, Fail("secret", purpose+" is empty")
	}
	return material, nil
}

func ReadWithPreferredSecrets[C any, R any](ctx Context, purposes []string, open func(SecretMaterial) (C, error), read func(C, string) (R, error), fallbackable func(error) bool) (R, string, error) {
	var zero R
	var failures []string
	for _, purpose := range purposes {
		purpose = strings.TrimSpace(purpose)
		if purpose == "" {
			continue
		}
		material, ok := ctx.OptionalSecret(purpose)
		if !ok {
			failures = append(failures, purpose+" unavailable")
			continue
		}
		material.Purpose = purpose
		client, err := open(material)
		if err != nil {
			failures = append(failures, purpose+" open: "+err.Error())
			continue
		}
		result, err := read(client, purpose)
		if err == nil {
			return result, purpose, nil
		}
		if fallbackable == nil || !fallbackable(err) {
			return zero, purpose, err
		}
		failures = append(failures, purpose+" read: "+err.Error())
	}
	if len(failures) == 0 {
		return zero, "", fmt.Errorf("no preferred secrets configured")
	}
	return zero, "", errors.New(strings.Join(failures, "; "))
}

func DefaultSecretGetter(ctx Context, purpose string) (SecretMaterial, error) {
	host := strings.TrimSpace(os.Getenv("DEX_HOST_CMD"))
	if host == "" {
		host = "dex"
	}
	plugin := strings.TrimSpace(ctx.Request.Plugin)
	if plugin == "" && ctx.plugin != nil {
		plugin = ctx.plugin.manifest.Name
	}
	args := []string{"secret", "get", plugin, "--instance", ctx.Request.Instance, "--grant", ctx.Request.Grant, "--purpose", purpose, "-o", "json"}
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
	if material.Purpose == "" {
		material.Purpose = purpose
	}
	return material, nil
}
