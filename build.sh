#!/bin/bash
set -eux

go mod verify 
go test ./...
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build --ldflags="-X pkg.version=$(git describe --dirty)"
