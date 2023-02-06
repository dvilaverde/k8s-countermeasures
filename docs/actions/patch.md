# Patch Action

This action will apply a [patch](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/)
to a Kubernetes object.

## Uses Cases

* Apply changes to increase CPU or memory limits of a pod temporarily.
* Alter environments variables in deployment or configMap to reconfigure the
application to work around an issue.

## Specification

```yaml
apiVersion: countermeasure.vilaverde.rocks/v1alpha1
kind: CounterMeasure
metadata:
  name: json-patch-action
spec:
  onEvent:
    name: HTTP_404
  actions:
  - name: custom-restart-pod
    patch:
      patchType: application/json-patch+json
      yamlTemplate: |
        - op: add
          path: /spec/template/spec/containers/0/env/-
          value: { "name": "ENV.3", "value": "value3" }
      targetObjectRef: 
        apiVersion: apps/v1
        kind: Deployment
        name: monitored-app
        namespace: ns-custom
```

The following properties are allowed under `patch`:

* `patchType`: One of `application/json-patch+json` [RFC 6902](https://www.rfc-editor.org/rfc/rfc6902),
`application/merge-patch+json`[RFC 7386](https://www.rfc-editor.org/rfc/rfc7386), or
`application/strategic-merge-patch+json`.
* `yamlTemplate`: Although patch types are defined as JSON, they are specified
here as YAML and will be converted to JSON before being applied.
* `targetObjectRef`: A reference to the `Object` that will be deleted:
  * `name`: The name of the pod.
  * `namespace`: The namespace where the `Pod` is running.
  * `kind`: The resource kind.
  * `apiVersion`: The group/version for the resource.

## Templating

The properties of `targetObjectRef` can include [Golang templates](https://pkg.go.dev/text/template).
See the [templating](templating.md) docs for more details.
