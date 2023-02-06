# Debug Action

This action will apply a strategic merge patch to a Pod, adding an [ephemeral container](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#ephemeral-container)
that can be used to run troubleshooting steps.

Essentially replicating the `kubectl debug` [command](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/),
for example:

```bash
kubectl debug -i --arguments-only \
  -c <debug-container-name> <pod-name> \
  --image=<image:tag> \
  --target=<container-name> \
  -- sh -c ls
```

## Uses Cases

* Collect debug information for containers that may experience intermittent
issues and don't package debugging tools for production images.
  * Use JDK debug image to collect thread dumps or heap dumps from container
  running JREs.
  * Running profiling tools automatically such as [Async Profiler](https://github.com/jvm-profiling-tools/async-profiler).

## Specification

```yaml
apiVersion: countermeasure.vilaverde.rocks/v1alpha1
kind: CounterMeasure
metadata:
  name: example-debug
spec:
  onEvent:
    name: HighCpuAlert
  actions:
  - name: debug-action-name
    debug:
      name: ephemeral-container-name
      command:
      - "touch"
      args:
      - "/proc/1/root/iwashere.txt"
      image: busybox:1.28
      stdin: true
      podRef: 
        name: "{{ .Data.pod }}"
        namespace: "{{ .Data.namespace }}"
        container: main
```

The following properties are allowed under `debug`:

* `name`: The name to use for the ephemeral container. If not provided the operator
will generate a unique name. If a name is provided and the debug action is executed
on the same pod more than once, then the subsequent exection will take no action.
* `command`: The command to execute within the debug container.
* `args`: The args for the command.
* `image`: Container image to use for debug container.
* `stdin`: Keep stdin open on the container(s) in the pod, even if nothing is attached.
* `tty`: Allocate a TTY for the debugging container.
* `podRef`: A reference to the `Pod` that the debug action will apply to.
  * `name`: The name of the pod.
  * `namespace`: The namespace where the `Pod` is running.
  * `container`: Targets processes in this container name.

## Templating

The properties of `podRef` can include [Golang templates](https://pkg.go.dev/text/template)
to be applied against the Event data structure. See the [templating](templating.md)
more details.
