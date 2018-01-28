#!/usr/bin/env bash

pushd $GOPATH/src/github.com/kubedb/admission-webhook/hack/gendocs
go run main.go
popd
