# Custom Resource Definitions

## CounterMeasure

In a `CounterMeasure`, multiple actions can be defined to be executed in document
order. See the documention for each action for more details.

## CounterMeasure Specification

```yaml
apiVersion: countermeasure.vilaverde.rocks/v1alpha1
kind: CounterMeasure
metadata:
  name: example-debug
spec:
  onEvent:
    name: AlertName
    suppressionPolicy:
      duration: 120s
    sourceSelector:
      matchLabels:
        app.kubernetes.io/name: p8s-source
        app.kubernetes.io/instance: dev
  actions:
  - name: name
    retryEnabled: false
    delete:
      << delete_spec >>
    patch:
      << patch_spec >>
    debug:
      << debug_spec >>
    restart:
      << restart_spec >>
```

* `onEvent`: defines the event that will trigger the countermeasures
  * `name`: the named event from the event source.
  * `suppressionPolicy`: (optional) policy that will be used to suppress duplicate
  events.
    * `duration`: currently the only policy, defines a `Duration` of time that
    duplicate events are ignored after first being received.
  * `matchLabels`: map of labels that are used to find
  event sources that this countermeasure is interested in events from. If not
  provided, all event sources will trigger this countermeasure.
* `actions`: an array of actions, each action will only have one of the 4 action
types (`delete`, `patch`, `debug`, `restart`) defined.
  * `name`: The name of the action used for logging and reporting in events.
  * `retryEnabled`: When set to true, the action will retry in the event of an error.
It is recommend that the action is idempotent when enabling this property.
  * `delete`: See [Delete Action](actions/delete.md)
  * `patch`: See [Patch Action](actions/patch.md)
  * `debug`: See [Debug Action](actions/debug.md)
  * `restart`: See [Restart Action](actions/restart.md)

## Prometheus

Currently Prometheus is the only built in event source. It is implmented to poll
the [Prometheus Alerts API](https://prometheus.io/docs/prometheus/latest/querying/api/#alerts)
and broadcast the events and labels to the event bus.

## Prometheus Specification

```yaml
apiVersion: eventsource.vilaverde.rocks/v1alpha1
kind: Prometheus
metadata:
  name: p8s-source-basic
  labels:
    app.kubernetes.io/name: p8s-source
    app.kubernetes.io/instance: dev
spec:
  service:
    name: prometheus-operated
    namespace: monitoring
  auth:
    secretRef:
      name: p8s-basic-auth
      namespace: ns-custom
  includePending: false
  pollingInterval: 30s
```

* `service`: defines the event that will trigger the countermeasures
  * `name`: the name of Kubernetes Service for the Prometheus pods.
  * `namespace`: the namespace where the Prometheus service is deployed.
  * `useTls`: (optional) true if the HTTPS endpoint should be used.
  * `port`: (optional) should be a valid port number (1-65535, inclusive).
  * `targetPort`: (optional) should be a valid name of a port in the target service
* `auth`: (optional) defines how to authenticate with Prometheus.
  * `secretRef`: references a `kubernetes.io/basic-auth` secret
    * `name`: the name of the Kubernetes Secret
    * `namespace`: the namespace of the Kubernetes Secret
  * `includePending`: true if pending alerts should be broadcast to the event bus
  and treated as active alerts.
  * `pollingInterval`: interval in which the Alerts API should be polled.
