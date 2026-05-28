package kubernetes

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "kubernetes"
	PluginVersion     = "0.1.0"
	PluginDescription = "Kubernetes cluster discovery using kubeconfig and kubectl."

	OperationClusterList      = "kubernetes.cluster.list"
	OperationClusterTest      = "kubernetes.cluster.test"
	OperationEndpointDiscover = "kubernetes.endpoint.discover"

	EndpointClusterDiscovered = "kubernetes.discovered_endpoints"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"k8s", "kube", PluginName},
		Operations: []core.OperationSpec{
			clusterListSpec(),
			clusterTestSpec(),
			endpointDiscoverSpec(),
		},
		Endpoints: []core.EndpointSpec{
			pluginbinding.Endpoint(EndpointClusterDiscovered, "Product endpoints discovered inside Kubernetes clusters.", "kubernetes", "prometheus", "loki", "homer", "mysql", "postgres"),
		},
	}
}

func clusterListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ClusterListInput, ClusterListResult](
		OperationClusterList,
		"List kubeconfig contexts.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func clusterTestSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ClusterTestInput, ClusterTestResult](
		OperationClusterTest,
		"Probe Kubernetes cluster reachability through kubeconfig.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func endpointDiscoverSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[EndpointDiscoverInput, EndpointDiscoverResult](
		OperationEndpointDiscover,
		"Discover product endpoints from Kubernetes services.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func kubernetesReadOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork, core.OperationEffectFilesystem),
		pluginbinding.Access(core.OperationAccessNetwork, core.OperationAccessFilesystem),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}
