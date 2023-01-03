#!/bin/zsh

set -e

POD_IP=$(kubectl -n ns-custom get pod -l app.kubernetes.io/name=monitored-app -o jsonpath='{.items[0].status.podIP}')
POD_ADDRESS="$(echo $POD_IP | tr . -).default.pod.cluster.local:8080"
kubectl run hey --rm -i --image demisto/rakyll-hey:1.0.0.40629 -- hey -m GET http://$POD_ADDRESS/err