#!/bin/bash
set -eux

go mod verify 
go test ./...
go build --ldflags="-X pkg.version=$(git describe --dirty)"
