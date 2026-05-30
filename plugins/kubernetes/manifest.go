package kubernetes

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

const (
	PluginName        = "kubernetes"
	PluginVersion     = "0.18.2"
	PluginDescription = "Kubernetes cluster discovery using kubeconfig and kubectl."

	OperationClusterList      = "kubernetes.cluster.list"
	OperationClusterTest      = "kubernetes.cluster.test"
	OperationEndpointDiscover = "kubernetes.endpoint.discover"
	OperationNamespaceList    = "kubernetes.namespace.list"
	OperationServiceList      = "kubernetes.service.list"
	OperationServiceShow      = "kubernetes.service.show"
	OperationPodList          = "kubernetes.pod.list"
	OperationPodShow          = "kubernetes.pod.show"
	OperationPodLogs          = "kubernetes.pod.logs"
	OperationPortForwardStart = "kubernetes.portforward.start"
	OperationPortForwardStop  = "kubernetes.portforward.stop"
	OperationDeploymentList   = "kubernetes.deployment.list"
	OperationDeploymentShow   = "kubernetes.deployment.show"
	OperationContainerList    = "kubernetes.container.list"
	OperationContainerShow    = "kubernetes.container.show"

	EndpointClusterDiscovered = "kubernetes.discovered_endpoints"
	DatasourceInventory       = "kubernetes.inventory"

	EntityResource   = "kubernetes.resource"
	EntityNamespace  = "kubernetes.namespace"
	EntityService    = "kubernetes.service"
	EntityPod        = "kubernetes.pod"
	EntityDeployment = "kubernetes.deployment"
	EntityContainer  = "kubernetes.container"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"kube", "k8s"},
		Operations: []core.OperationSpec{
			clusterListSpec(),
			clusterTestSpec(),
			endpointDiscoverSpec(),
			namespaceListSpec(),
			serviceListSpec(),
			serviceShowSpec(),
			podListSpec(),
			podShowSpec(),
			podLogsSpec(),
			portForwardStartSpec(),
			portForwardStopSpec(),
			deploymentListSpec(),
			deploymentShowSpec(),
			containerListSpec(),
			containerShowSpec(),
		},
		Datasources: []core.DatasourceSpec{
			inventoryDatasourceSpec(),
		},
		Endpoints: []core.EndpointSpec{
			pluginbinding.Endpoint(EndpointClusterDiscovered, "Product endpoints discovered inside Kubernetes clusters.", "kubernetes", "prometheus", "loki", "homer", "mysql", "postgres"),
		},
		Metadata: map[string]string{
			pluginbinding.ManifestProtocolKey: protocol.Version,
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

func namespaceListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, NamespaceListResult](
		OperationNamespaceList,
		"List Kubernetes namespaces.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func serviceListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, ServiceListResult](
		OperationServiceList,
		"List Kubernetes services.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func serviceShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, ServiceShowResult](
		OperationServiceShow,
		"Show one Kubernetes service.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func podListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, PodListResult](
		OperationPodList,
		"List Kubernetes pods.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func podShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, PodShowResult](
		OperationPodShow,
		"Show one Kubernetes pod.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func podLogsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PodLogsInput, PodLogsResult](
		OperationPodLogs,
		"Read bounded logs for one Kubernetes pod with optional time bounds.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func portForwardStartSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PortForwardStartInput, PortForwardResult](
		OperationPortForwardStart,
		"Start a managed kubectl port-forward for a Kubernetes service, pod, or deployment.",
		pluginbinding.Effects(core.OperationEffectWrite),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	)
}

func portForwardStopSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PortForwardStopInput, PortForwardStopResult](
		OperationPortForwardStop,
		"Stop a managed kubectl port-forward by ID or process group.",
		pluginbinding.Effects(core.OperationEffectWrite),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationIdempotent),
	)
}

func deploymentListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, DeploymentListResult](
		OperationDeploymentList,
		"List Kubernetes deployments.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func deploymentShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, DeploymentShowResult](
		OperationDeploymentShow,
		"Show one Kubernetes deployment.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func containerListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, ContainerListResult](
		OperationContainerList,
		"List Kubernetes containers derived from pods.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func containerShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, ContainerShowResult](
		OperationContainerShow,
		"Show one Kubernetes container derived from a pod.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func inventoryDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[InventorySearchInput, InventorySearchResult](
		DatasourceInventory,
		EntityResource,
		"Kubernetes namespaces, services, pods, deployments, and containers.",
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.DatasourceAccess(core.OperationAccessProvider),
		pluginbinding.Completion(
			"Kubernetes contexts, endpoints, namespaces, resource names, pod names, container names, and labels.",
			"endpoint_ref",
			"context",
			"namespace",
			"id",
			"title",
			"name",
			"pod",
			"containers",
			"labels",
		),
	)
}

func kubernetesReadOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.Effects(core.OperationEffectRead),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}
