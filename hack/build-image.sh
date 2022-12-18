export IMG=k8s-countermeasures-operator:v0.1 

make docker-build
minikube image load k8s-countermeasures-operator:v0.1
make deploy

