# Delete Action

This action will delete a Kubernetes Object, for example a `Pod`.

Essentially replicating the `kubectl delete` command,
for example:

```bash
kubectl delete pod unwanted
```

## Uses Cases

* Removing pods that may be stuck in an unwanted state.
* Deleting pods that are passing liveness and readiness probes but otherwise
exhibiting undesired behavior where restart might resolve the issue.

## Specification

```yaml
apiVersion: countermeasure.vilaverde.rocks/v1alpha1
kind: CounterMeasure
metadata:
  name: delete-action
spec:
  onEvent:
    name: HighCpuAlert
  actions:
  - name: delete-pod
    delete:
      targetObjectRef: 
        name: "{{ .Data.pod }}"
        namespace: "{{ .Data.namespace }}"
        kind: Pod
        apiVersion: v1
```

The following properties are allowed under `delete`:

* `targetObjectRef`: A reference to the `Object` that will be deleted:
  * `name`: The name of the pod.
  * `namespace`: The namespace where the `Pod` is running.
  * `kind`: The resource kind.
  * `apiVersion`: The group/version for the resource.

## Templating

The properties of `targetObjectRef` can include [Golang templates](https://pkg.go.dev/text/template).
See the [templating](templating.md) docs for more details.
