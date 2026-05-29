package asterisk

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

const (
	PluginName        = "asterisk"
	PluginVersion     = "0.16.0"
	PluginDescription = "Asterisk endpoint discovery and AMI operations."

	EnvAsteriskAMIUsername = "ASTERISK_AMI_USERNAME"
	EnvAsteriskAMISecret   = "ASTERISK_AMI_SECRET"
	EnvAsteriskAMIPassword = "ASTERISK_AMI_PASSWORD"

	AuthMethodAMI       = "ami"
	AuthPurposeUsername = "username"
	AuthPurposeSecret   = "secret"
	AuthPurposePassword = "password"

	OperationAMIPing = "asterisk.ami.ping"

	EndpointAsterisk = "asterisk.endpoints"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"pbx", PluginName},
		Operations: []core.OperationSpec{
			amiPingSpec(),
		},
		Auth: []core.AuthMethod{{
			Name:        AuthMethodAMI,
			Kind:        "username_secret",
			Description: "Asterisk Manager Interface username and secret.",
			Env:         []string{EnvAsteriskAMIUsername, EnvAsteriskAMISecret, EnvAsteriskAMIPassword},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeUsername, "AMI username", true, false, EnvAsteriskAMIUsername),
				pluginbinding.AuthField(AuthPurposeSecret, "AMI secret", true, true, EnvAsteriskAMISecret),
				pluginbinding.AuthField(AuthPurposePassword, "AMI password alias", false, true, EnvAsteriskAMIPassword),
			},
		}},
		Endpoints: []core.EndpointSpec{
			pluginbinding.Endpoint(EndpointAsterisk, "Asterisk AMI endpoints discovered from Kubernetes services, secrets, and config maps.", "asterisk", "ami", "asterisk-ami"),
		},
		Metadata: map[string]string{
			pluginbinding.ManifestProtocolKey: protocol.Version,
		},
	}
}

func amiPingSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AMIPingInput, AMIPingResult](
		OperationAMIPing,
		"Ping an Asterisk Manager Interface endpoint.",
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(AuthPurposeUsername, AuthPurposeSecret, AuthPurposePassword),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessProvider, core.OperationAccessAuth, core.OperationAccessSecret),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	)
}
