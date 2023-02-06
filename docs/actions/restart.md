# Restart Action

This action will restart a Kubernetes `Deployment`.

Essentially replicating the `kubectl rollout restart` command,
for example:

```bash
kubectl -n ns-custom rollout restart deployment/monitored-app
```

## Uses Cases

* Triggering a rolling restart after changing a configMap in a prior action.

## Specification

```yaml
apiVersion: countermeasure.vilaverde.rocks/v1alpha1
kind: CounterMeasure
metadata:
  name: restart-action
spec:
  onEvent:
    name: HTTP_404
  actions:
  - name: restart-pods
    restart:
      deploymentRef: 
        name: monitored-app
        namespace: ns-custom
```

The following properties are allowed under `restart`:

* `deploymentRef`: A reference to the `Object` that will be deleted:
  * `name`: The name of the deployment.
  * `namespace`: The namespace where the `Deployment` is deployed.

## Templating

The properties of `deploymentRef` can include [Golang templates](https://pkg.go.dev/text/template).
See the [templating](templating.md) docs for more details.
