#!/bin/bash
set -x

kubectl delete deployment -l app=kubedb -n kube-system
kubectl delete service -l app=kubedb -n kube-system
kubectl delete secret -l app=kubedb -n kube-system

# Delete RBAC objects, if --rbac flag was used.
kubectl delete serviceaccount -l app=kubedb -n kube-system
kubectl delete clusterrolebindings -l app=kubedb -n kube-system
kubectl delete clusterrole -l app=kubedb -n kube-system
