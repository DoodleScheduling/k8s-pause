# k8s-pause

[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/4787/badge)](https://bestpractices.coreinfrastructure.org/projects/4787)
[![e2e](https://github.com/DoodleScheduling/k8s-pause/workflows/e2e/badge.svg)](https://github.com/DoodleScheduling/k8s-pause/actions)
[![report](https://goreportcard.com/badge/github.com/DoodleScheduling/k8s-pause)](https://goreportcard.com/report/github.com/DoodleScheduling/k8s-pause)
[![license](https://img.shields.io/github/license/DoodleScheduling/k8s-pause.svg)](https://github.com/DoodleScheduling/k8s-pause/blob/master/LICENSE)
[![release](https://img.shields.io/github/release/DoodleScheduling/k8s-pause/all.svg)](https://github.com/DoodleScheduling/k8s-pause/releases)

Suspend and resume entire kubernetes namespaces! \
This controller makes this missing feature possible.

![howto](./examples/screen.gif)

## Example Usage

Suspend:
```
kubectl annotate ns/my-namespace k8s-pause/suspend=true --overwrite
```

Resume:
```
kubectl annotate ns/my-namespace k8s-pause/suspend=false --overwrite
```

## Details

The suspend flag on namespace level will affect only but any pods. It will not touch any resources besides pods.
However it guarantees that no pod will be scheduled if the namespace is suspended no matter from where and how the pod is created.
Once the namespace is resumed it will schedule all suspended pods.

There is no reason to downscale deployments, statefulsets or any other kind of workloads, k8s-pause will handle any workloads within a namespace.


## Installation

### Requirements
Currently it is required to have [certmanager](https://cert-manager.io/docs/installation/) deployed on the cluster with either kustomize or helm deployment.

## Bypass namespace
The controller will intercept all pod write communication. The namespace which hosts k8s-pause needs to be bypassed otherwise you won't be able to create
pods anymore!
By default you can annotate the namespace using:
```
kubectl annotate ns/my-namespace control-plane=controller-manager
```

Both kustomize and helm deployments will have this exception by default. You can configure a different rule in each way of deployment.
**Note**: It is also good practice to have other namespaces bypassed which should not support k8s-pause. For instance `kube-system` is a good example.

### Helm

Please see [chart/k8s-pause](https://github.com/DoodleScheduling/k8s-pause/tree/master/chart/k8s-pause) for the helm chart docs.

### Manifests/kustomize

Alternatively you may get the bundled manifests in each release to deploy it using kustomize or use them directly.

## Configure the controller

You may change base settings for the controller using env variables (or alternatively command line arguments).
It is possible to set defaults (fallback values) for the vault address and also all TLS settings.

Available env variables:

| Name  | Description | Default |
|-------|-------------| --------|
| `METRICS_ADDR` | The address of the metric endpoint binds to. | `:9556` |
| `PROBE_ADDR` | The address of the probe endpoints binds to. | `:9557` |
| `ENABLE_LEADER_ELECTION` | Enable leader election for controller manager. | `false` |
| `LEADER_ELECTION_NAMESPACE` | Change the leader election namespace. This is by default the same where the controller is deployed. | `` |
| `NAMESPACES` | The controller listens by default for all namespaces. This may be limited to a comma delimited list of dedicated namespaces. | `` |
| `CONCURRENT` | The number of concurrent reconcile workers.  | `2` |
