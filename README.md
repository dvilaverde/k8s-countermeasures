# k8s-countermeasures

[![Build Status](https://github.com/dvilaverde/k8s-countermeasures/workflows/build/badge.svg)](https://github.com/dvilaverde/k8s-countermeasures/actions)

## Actions

### Delete

Deletes a Object with a given name in a given namespace.

### Patch

Applies a Patch to an Object with a given name in a given namespace.

### Restart

Restarts a Deployment with a given name in a given namespace.

### Debug

Will apply a strategic merge patch to a Pod, adding an [ephemeral container](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#ephemeral-container) that can be used to run troubleshooting steps.

Essentially replicating the `kubectl debug` command like:

```bash
kubectl debug -i --arguments-only -c <debug-container-name> <pod-name> --image=<image:tag> --target=<container-name> -- sh -c ls -l
```


## Debugging

To debug the controller locally against a running K8s cluster, add this entry to the
`/etc/hosts` file so that the operator can communicate with Prometheus

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
