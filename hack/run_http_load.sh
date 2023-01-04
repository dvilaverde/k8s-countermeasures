#!/bin/zsh

set -e
POD_ADDRESS="monitored-app.ns-custom.svc.cluster.local:8080"
kubectl run hey --rm -i --image demisto/rakyll-hey:1.0.0.40629 -- hey -m GET http://$POD_ADDRESS/err