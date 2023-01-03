#!/bin/zsh

set -e

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
sleep 5
./run_http_load.sh