#!/bin/zsh

set -e

export IMG=k8s-countermeasures-operator:v0.1 

make docker-build
kind load docker-image k8s-countermeasures-operator:v0.1 --name local-cluster
make deploy

