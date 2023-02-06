# Kubernetes CounterMeasures

[![Build Status](https://github.com/dvilaverde/k8s-countermeasures/workflows/build/badge.svg)](https://github.com/dvilaverde/k8s-countermeasures/actions)

**Project status: *alpha*** Not all planned features are completed. The API, spec,
status and other user facing objects may change, but in a backward compatible way.

Packaging scripts and instructions for deployment are still in progress
and looking for contributors.

## Overview

This project aims to define a API and controller in Kubernetes to codify
project runbooks, allowing for automation of actions that are manually
taken when on on-call engineer receives an alert.

For example, imagine a Java application with a runbook that defines when an alert
for high CPU is received, the on-call engineer is to take a thread-dump for analysis.
Doing this manually may prove difficult depending on how long the high CPU event
lasts and the engineer availability, and whether or not the
container has the debug tools required.

This project allows for the automation of the above runbook task by using an operator
written using the [OperatorSDK](https://sdk.operatorframework.io) and a few CRDs
to define the `event` to monitor and the `actions` to take.

The operator allows for deployment of an event source, currently only Prometheus
is supported, and a countermeasure that defines one or more actions. The event source
will publish events into an internal event bus to be conssumed by the countermeasures.

For more detailed instructions and [examples](config/samples/) see the [README](docs/README.md) in
the [docs](docs) folder.

## Prerequisites

The Kubernetes CounterMeasures Operator uses [Ephemeral Containers](https://v1-25.docs.kubernetes.io/docs/concepts/workloads/pods/ephemeral-containers/)
which was *alpha* in Kubernetes `1.22.0`, *beta* in `1.23.0`, and stable in `>=1.25.0`.
Therefore it is recommended to use verion `>=1.25.0`, but development and testing
was done with a Kubernetes cluster of version `>=1.23.0`.

## CustomResourceDefinitions

A core feature of the Kubernetes CounterMeasures Operator is to monitor
the Kubernetes API server for changes to specific objects and ensure that
your application is monitored for any undesirable conditions and when detected
the appropriate actions are taken as a counter measure.
The Operator acts on the following [custom resource definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions/):

* **`CounterMeasure`**, which defines a condition to watch for and actions to take
when it occurs.
* **`Prometheus`**, which defines an event source that trigger the counter measures.

The Kubernetes CounterMeasures operator automatically detects changes
in the Kubernetes API server to any of the above objects, and ensures
your the monitors are updated.

To learn more about the CRDs introduced by the Kubernetes CounterMeasures Operator
have a look at the [documentation](docs/actions.md).

## Dynamic Admission Control

To provide validation an [admission webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
is provided to validate CRD resources upon initial creation or update
or during dry run.

For more information on this feature, see the [user guide](docs/webhook.md).

## Quickstart

To quickly try out the Kubernetes CounterMeasures Operator inside a [Kind](https://kind.sigs.k8s.io)
cluster, run the following command:

```bash
./hack/start-cluster.sh
make install
make deploy
```

To run the Operator outside of a cluster instead of running `make deploy`, use:

```bash
make run
```

## Removal

To remove the operator, first delete any custom resources you created in each namespace.

```bash
for n in $(kubectl get namespaces -o jsonpath={..metadata.name}); do
  kubectl delete --all --namespace=$n countermeasure
done
```

After a couple of minutes you can go ahead and remove the operator itself.

```bash
make undeploy
make uninstall
```

## Development

### Prerequisites

* golang environment
* docker (used for creating container images, etc.)
* kind (optional)

### Testing

#### Running *unit tests*

`make test`

### Debugging

To debug the controller locally against a running K8s cluster, add this entry to
the `/etc/hosts` file so that the operator can communicate with Prometheus.

```text
##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1 localhost
# Add for k8s-countermeasures debugging
127.0.0.1 prometheus-operated.monitoring.svc 
```

then enable port forwarding from the development host to the promtheus service:

```bash
kubectl -n monitoring port-forward service/prometheus-operated 9090:9090
```

## Contributing

Many files (documentation, manifests, ...) in this repository are
auto-generated. Before proposing a pull request:

1. Commit your changes.
2. Run `make generate`.
3. Commit the generated changes.

## Security

If you find a security vulnerability related to the Kubernetes CounterMeasures
Operator, please do not report it by opening a GitHub issue, but instead please
send an e-mail to the owner of this project.
