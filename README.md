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
kubectl annotate ns/my-namespace k8s-pause/suspend="true" --overwrite
```

Resume:
```
kubectl annotate ns/my-namespace k8s-pause/suspend="false" --overwrite
```

## Helm chart

Please see [chart/k8s-pause](https://github.com/DoodleScheduling/k8s-pause) for the helm chart docs.

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
| `CONCURRENT` | The number of concurrent reconcile workers.  | `4` |
