#!/bin/zsh

SCRIPT_DIR=${0:a:h}

# Create a local K8s cluster with 1 control-plane node and 1 worker node
kind create cluster -n local-cluster --config $SCRIPT_DIR/kind/k8s-local-config.yaml

# Install Prometheus Operator
kubectl create namespace monitoring
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
helm -n monitoring install test-prom-op bitnami/kube-prometheus \
    --set alertmanager.enabled=false \
    --set exporters.node-exporter.enabled=false \
    --set exporters.kube-state-metrics.enabled=false \
    --set blackboxExporter.enabled=false

# Deploy the example app
kubectl create namespace ns-custom
kubectl -n ns-custom apply -f $SCRIPT_DIR/bad_app/monitored_app.yaml

# Next trigger a firing alert by running a shell in the cluster and running a load test
# but you'll need the pod ip address
#
POD_IP=$(kubectl -n ns-custom get pod -l app.kubernetes.io/name=monitored-app -o jsonpath='{.items[0].status.podIP}')
POD_ADDRESS="$(echo $POD_IP | tr . -).default.pod.cluster.local:8080"
kubectl run hey --rm -i --image demisto/rakyll-hey:1.0.0.40629 -- hey -m GET http://$POD_ADDRESS/err