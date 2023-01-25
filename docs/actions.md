# Actions

In a `CounterMeasure`, multiple actions can be defined to be executed in the order
they are defined. See the documention for each action for more details.

## Specification

```yaml
apiVersion: countermeasure.vilaverde.rocks/v1alpha1
kind: CounterMeasure
metadata:
  name: example-debug
spec:
  onEvent:
    name: AlertName
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

* `name`: The name of the action used for logging and reporting in events.
* `retryEnabled`: When set to true, the action will retry in the event of an error.
It is recommend that the action is idempotent when enabling this property.
* `delete`: See [Delete Action](actions/delete.md)
* `patch`: See [Patch Action](actions/patch.md)
* `debug`: See [Debug Action](actions/debug.md)
* `restart`: See [Restart Action](actions/restart.md)
