# Endpoint Flow

Endpoints are host-owned references to reachable systems. Discovery finds
candidates, import/register stores the chosen endpoints, and test updates the
endpoint's last known health.

## Kubernetes Cluster

Discover Kubernetes cluster endpoints from kubeconfig contexts:

```bash
dex endpoint discover kubernetes --plugin kubernetes -o json
```

Import one discovered candidate:

```bash
dex endpoint discover kubernetes --plugin kubernetes -o json > /tmp/dex-k8s.json
dex endpoint import --from /tmp/dex-k8s.json --candidate 1 --id dev-kubernetes
```

Probe the registered cluster endpoint:

```bash
dex endpoint test dev-kubernetes -o json
dex endpoint show dev-kubernetes -o json
```

The test uses `kubernetes.cluster.test`, which calls the Kubernetes API server
through client-go and stores the result as `last_health` on the endpoint.

Run health checks for every registered endpoint:

```bash
dex doctor endpoints -o json
```

You can scope the check to one product:

```bash
dex doctor endpoints kubernetes -o json
```

## Cluster Inventory

Once the Kubernetes plugin can reach a cluster, use read-only operations to
inspect inventory:

```bash
dex op run kubernetes.namespace.list '{"endpoint_ref":"dev-kubernetes"}'
dex op run kubernetes.service.list '{"endpoint_ref":"dev-kubernetes","namespace":"latest"}'
dex op run kubernetes.pod.list '{"endpoint_ref":"dev-kubernetes","namespace":"latest"}'
```

Show one resource:

```bash
dex op run kubernetes.service.show '{"endpoint_ref":"dev-kubernetes","namespace":"latest","name":"api"}'
dex op run kubernetes.pod.show '{"endpoint_ref":"dev-kubernetes","namespace":"latest","name":"api-123"}'
```

The same operations are available through shortcut bindings:

```bash
dex kube ns ls --endpoint dev-kubernetes
dex kube svc ls --endpoint dev-kubernetes --namespace latest
dex kube svc show latest/api --endpoint dev-kubernetes
dex kube pod ls --endpoint dev-kubernetes --namespace latest --query api
```

These shortcuts are resolved from marketplace metadata and call the same
underlying `kubernetes.*` operations shown above.

Datasource search exposes namespaces, services, and pods as common datasource
records:

```bash
dex search --plugin kubernetes api
```

## In-Cluster Product Discovery

Use a reachable cluster to discover product endpoints inside it:

```bash
dex endpoint discover mysql \
  --plugin kubernetes \
  --context arn:aws:eks:eu-central-1:123456789012:cluster/dev \
  --namespace latest \
  -o json
```

Import a selected database candidate:

```bash
dex endpoint import --from /tmp/dex-mysql.json --candidate 1 --id latest-mysql
```

Then query through the SQL plugin without copying credentials into dex state:

```bash
dex op run sql.query '{"endpoint_ref":"latest-mysql","query":"select 1 as ok","max_rows":1}'
```

For Kubernetes-discovered SQL endpoints, credentials can remain intrinsic to the
cluster through a Kubernetes secret reference.
