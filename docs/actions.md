# Actions

## Delete

Deletes a Object with a given name in a given namespace.

## Patch

Applies a Patch to an Object with a given name in a given namespace.

## Restart

Restarts a Deployment with a given name in a given namespace.

## Debug

Will apply a strategic merge patch to a Pod, adding an [ephemeral container](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#ephemeral-container)
that can be used to run troubleshooting steps.

Essentially replicating the `kubectl debug` command like:

```bash
kubectl debug -i --arguments-only \
  -c <debug-container-name> <pod-name> \
  --image=<image:tag> \
  --target=<container-name> \
  -- sh -c ls -l
```
