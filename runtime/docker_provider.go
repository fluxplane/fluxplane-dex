package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/internal/dockerhost"
)

func (r Runner) hostDockerProviderCall(ctx context.Context, _ string, _ string, _ string, input pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	action := strings.TrimSpace(input.Action)
	if action == "" || action == "Close" {
		return pluginbinding.ProviderCallResponse{}, fmt.Errorf("unsupported docker provider action %q", input.Action)
	}
	client, err := dockerhost.NewClient()
	if err != nil {
		return pluginbinding.ProviderCallResponse{}, err
	}
	defer client.Close()
	method := reflect.ValueOf(client).MethodByName(action)
	if !method.IsValid() {
		return pluginbinding.ProviderCallResponse{}, fmt.Errorf("unsupported docker provider action %q", action)
	}
	args := []reflect.Value{reflect.ValueOf(ctx)}
	methodType := method.Type()
	switch methodType.NumIn() {
	case 1:
	case 2:
		argType := methodType.In(1)
		argValue := reflect.New(argType)
		if len(input.Payload) > 0 && string(input.Payload) != "null" {
			if err := json.Unmarshal(input.Payload, argValue.Interface()); err != nil {
				return pluginbinding.ProviderCallResponse{}, err
			}
		}
		args = append(args, argValue.Elem())
	default:
		return pluginbinding.ProviderCallResponse{}, fmt.Errorf("unsupported docker provider action %q", action)
	}
	results := method.Call(args)
	if len(results) != 2 {
		return pluginbinding.ProviderCallResponse{}, fmt.Errorf("unsupported docker provider action %q", action)
	}
	if errValue := results[1]; !errValue.IsNil() {
		if err, ok := errValue.Interface().(error); ok {
			return pluginbinding.ProviderCallResponse{}, err
		}
		return pluginbinding.ProviderCallResponse{}, fmt.Errorf("docker provider action %q failed", action)
	}
	result, err := json.Marshal(results[0].Interface())
	if err != nil {
		return pluginbinding.ProviderCallResponse{}, err
	}
	return pluginbinding.ProviderCallResponse{Result: result}, nil
}
