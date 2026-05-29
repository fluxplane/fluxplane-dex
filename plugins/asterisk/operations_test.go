package asterisk

import (
	"encoding/json"
	"testing"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

func TestAMIPingDelegatesToHostProvider(t *testing.T) {
	plugin := NewPluginWithService(Service{
		ProviderCall: func(_ pluginbinding.Context, action string, input any) (json.RawMessage, error) {
			if action != "ami.ping" {
				t.Fatalf("action = %q", action)
			}
			pingInput := input.(AMIPingInput)
			if pingInput.EndpointRef != "pbx-dev" {
				t.Fatalf("endpoint_ref = %q", pingInput.EndpointRef)
			}
			return json.RawMessage(`{"endpoint_ref":"pbx-dev","url":"ami://asterisk.latest.svc:5038","ok":true,"authenticated":true,"pong":true}`), nil
		},
	})

	out := plugintest.RunOK[AMIPingResult](t, plugin, OperationAMIPing, map[string]any{"endpoint_ref": "pbx-dev"})
	if !out.OK || !out.Authenticated || !out.Pong {
		t.Fatalf("out = %#v", out)
	}
}

func TestEndpointDiscoverFindsAMIServiceAndSecret(t *testing.T) {
	plugin := NewPluginWithService(Service{ProviderCall: fakeKubernetesProvider(t, map[string]json.RawMessage{
		"services": json.RawMessage(`[
			{"metadata":{"name":"asterisk","namespace":"latest","labels":{"app":"asterisk"}},"spec":{"type":"ClusterIP","ports":[{"name":"ami","port":5038}]}}
		]`),
		"secrets": json.RawMessage(`[
			{"metadata":{"name":"asterisk-ami","namespace":"latest"},"data":{"username":"ZGV4","secret":"c2VjcmV0"}}
		]`),
		"configmaps": json.RawMessage(`[]`),
	})})

	resp := plugin.Handle(request(t, protocol.CommandEndpointsDiscover, map[string]any{"product": "asterisk", "context": "dev", "namespace": "latest"}))
	if !resp.OK {
		t.Fatalf("response = %#v", resp.Error)
	}
	var discovery EndpointDiscoverResult
	if err := json.Unmarshal(resp.Result, &discovery); err != nil {
		t.Fatal(err)
	}
	if len(discovery.Candidates) != 1 {
		t.Fatalf("candidates = %#v", discovery.Candidates)
	}
	candidate := discovery.Candidates[0]
	if candidate.URL != "ami://asterisk.latest.svc:5038" || candidate.Protocol != "ami" || candidate.Product != "asterisk" {
		t.Fatalf("candidate = %#v", candidate)
	}
	if candidate.CredentialRef != "kubernetes://latest/secrets/asterisk-ami?context=dev" {
		t.Fatalf("credential_ref = %q", candidate.CredentialRef)
	}
}

func TestEndpointDiscoverFindsManagerConfConfigMap(t *testing.T) {
	plugin := NewPluginWithService(Service{ProviderCall: fakeKubernetesProvider(t, map[string]json.RawMessage{
		"services": json.RawMessage(`[
			{"metadata":{"name":"pbx","namespace":"latest","labels":{"app":"asterisk"}},"spec":{"ports":[{"name":"manager","port":5038}]}}
		]`),
		"secrets": json.RawMessage(`[]`),
		"configmaps": json.RawMessage(`[
			{"metadata":{"name":"asterisk-config","namespace":"latest"},"data":{"manager.conf":"[general]\nenabled = yes\n[dex]\nsecret = topsecret\nread = system\n"}}
		]`),
	})})

	resp := plugin.Handle(request(t, protocol.CommandEndpointsDiscover, map[string]any{"product": "ami", "context": "dev", "namespace": "latest"}))
	if !resp.OK {
		t.Fatalf("response = %#v", resp.Error)
	}
	var discovery EndpointDiscoverResult
	if err := json.Unmarshal(resp.Result, &discovery); err != nil {
		t.Fatal(err)
	}
	if len(discovery.Candidates) != 1 {
		t.Fatalf("candidates = %#v", discovery.Candidates)
	}
	if discovery.Candidates[0].CredentialRef != "kubernetes://latest/configmaps/asterisk-config?context=dev" {
		t.Fatalf("credential_ref = %q", discovery.Candidates[0].CredentialRef)
	}
}

func TestEndpointDiscoverIgnoresGenericDatabaseSecrets(t *testing.T) {
	plugin := NewPluginWithService(Service{ProviderCall: fakeKubernetesProvider(t, map[string]json.RawMessage{
		"services": json.RawMessage(`[]`),
		"secrets": json.RawMessage(`[
			{"metadata":{"name":"crossplane-provider-sql-db-secret-user-latest-freepbx","namespace":"latest","annotations":{"sealedsecrets.bitnami.com/cluster-wide":"true"}},"data":{"host":"ZGIuZXhhbXBsZS5jb20=","port":"NTQzMg==","username":"YXBw","password":"c2VjcmV0"}}
		]`),
		"configmaps": json.RawMessage(`[]`),
	})})

	resp := plugin.Handle(request(t, protocol.CommandEndpointsDiscover, map[string]any{"product": "asterisk", "context": "dev", "namespace": "latest"}))
	if !resp.OK {
		t.Fatalf("response = %#v", resp.Error)
	}
	var discovery EndpointDiscoverResult
	if err := json.Unmarshal(resp.Result, &discovery); err != nil {
		t.Fatal(err)
	}
	if len(discovery.Candidates) != 0 {
		t.Fatalf("candidates = %#v", discovery.Candidates)
	}
}

func TestEndpointDiscoverFindsExplicitAMISecretEndpoint(t *testing.T) {
	plugin := NewPluginWithService(Service{ProviderCall: fakeKubernetesProvider(t, map[string]json.RawMessage{
		"services": json.RawMessage(`[]`),
		"secrets": json.RawMessage(`[
			{"metadata":{"name":"pbx-ami","namespace":"latest"},"data":{"ami_host":"cGJ4LmV4YW1wbGUuY29t","ami_port":"NTAzOA==","ami_username":"ZGV4","ami_secret":"c2VjcmV0"}}
		]`),
		"configmaps": json.RawMessage(`[]`),
	})})

	resp := plugin.Handle(request(t, protocol.CommandEndpointsDiscover, map[string]any{"product": "asterisk", "context": "dev", "namespace": "latest"}))
	if !resp.OK {
		t.Fatalf("response = %#v", resp.Error)
	}
	var discovery EndpointDiscoverResult
	if err := json.Unmarshal(resp.Result, &discovery); err != nil {
		t.Fatal(err)
	}
	if len(discovery.Candidates) != 1 || discovery.Candidates[0].URL != "ami://pbx.example.com:5038" {
		t.Fatalf("candidates = %#v", discovery.Candidates)
	}
}

func fakeKubernetesProvider(t *testing.T, responses map[string]json.RawMessage) func(pluginbinding.Context, string, any) (json.RawMessage, error) {
	t.Helper()
	return func(_ pluginbinding.Context, action string, _ any) (json.RawMessage, error) {
		raw, ok := responses[action]
		if !ok {
			t.Fatalf("unexpected action %q", action)
		}
		return raw, nil
	}
}

func request(t *testing.T, command string, input any) protocol.Request {
	t.Helper()
	req, err := protocol.NewRequest(command, PluginName, input)
	if err != nil {
		t.Fatal(err)
	}
	return req
}
