# k8s-countermeasures




## Debugging

To debug the controller locally against a running K8s cluster, add this entry to the
`/etc/hosts` file so that the operator can communicate with Prometheus

```
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
```
kubectl -n monitoring port-forward service/prometheus-operated 9090:9090
```