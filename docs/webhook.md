# Admission Webhook

This guide describes how to deploy and use the CounterMeasure operator's admission
webhook service.

The admission webhook service is able to validate requests ensuring that
`CounterMeasure` and `Prometheus` objects are semantically valid.

This guide assumes that the CounterMeasures Operator has beed deployed
and that you've enabled [admission controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#how-do-i-turn-on-an-admission-controller)
in your cluster.

## Prerequisites

The Kubernetes API server expects admission webhook services to communicate
over HTTPS so we need:

1. Valid TLS certificate and key provisioned for the admission webhook service.
2. Kubernetes Secret containing the TLS certificate and key.

It is recommended to use [cert-manager](https://cert-manager.io/)
which manages both the lifecycle of the TLS certificates and the integration
with the Kubernetes API with respect to the webhook configuration (e.g.
automatic injection of the CA bundle).

TODO: document enabling admission webhook.
