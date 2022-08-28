#!/bin/bash
#    This file is part of conntrack-stats-exporter.
#
#    conntrack-stats-exporter is free software: you can redistribute it and/or
#    modify it under the terms of the GNU General Public License as published
#    by the Free Software Foundation, either version 3 of the License, or (at
#    your option) any later version.
#
#    conntrack-stats-exporter is distributed in the hope that it will be
#    useful, but WITHOUT ANY WARRANTY; without even the implied warranty of
#    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General
#    Public License for more details.
#
#    You should have received a copy of the GNU General Public License along
#    with conntrack-stats-exporter.  If not, see
#    <http://www.gnu.org/licenses/>.

set -eux

go mod verify 

if [ "$(lscpu | grep Architecture | awk '{print $2}')" = "aarch64" ]; then
    echo "arm64"
    export GOARCH=arm64

    CGO_ENABLED=0 go test ./...
else
    echo "amd64"
    export GOARCH=amd64

    go test --race ./...
fi

CGO_ENABLED=0 go build --ldflags="-X pkg.version=$(git describe --dirty)"
