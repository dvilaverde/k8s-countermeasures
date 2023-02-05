# Templating

Actions support [Golang templates](https://pkg.go.dev/text/template) to be applied
before the action is executed using the data from the event.

The properties of `podRef` can include [Golang templates](https://pkg.go.dev/text/template)
to be applied against the Event data structure. For example given this event:

```json
{
    "Name": "HighCpuAlert",
    "ActiveTime": "2022-11-14 02:45:16 +0000 UTC",
    "Data": {
        "pod": "hello-world-app-szdfh",
        "namespace": "my-team-ns"
    },
    "Source": {
        "Name": "prom-operated",
        "Namespace" "monitoring-ns"
    }
}
```
The `Data` property is defined as `map[string]string` and will contain all the properties
from the event as provided by the event provider.

Using the `podRef` property, from the [Debug](debug.md) action specification
as an example:

```yaml
      podRef: 
        name: "{{ .Data.pod }}"
        namespace: "{{ .Data.namespace }}"
        container: main
```

coupled with the event data provided above, the result of the evaluation would be:

```yaml
podRef: 
  name: hello-world-app-szdfh
  namespace: my-team-ns
  container: main
```
