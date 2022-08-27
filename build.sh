#!/bin/bash
set -eux

go mod verify 

if [ "$(lscpu | grep Architecture | awk '{print $2}')" = "aarch64" ]; then
    echo "arm64"
    export GOARCH=arm64

    go test ./...
else
    echo "amd64"
    export GOARCH=amd64

    go test --race ./...
fi

export CGO_ENABLED=0

go build --ldflags="-X pkg.version=$(git describe --dirty)"
