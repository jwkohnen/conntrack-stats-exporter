#!/bin/sh
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

set -eu

if [ "${CONNTRACK_STATS_EXPORTER_SLEEP:-}" != "" ]; then
  sleep "${CONNTRACK_STATS_EXPORTER_SLEEP}"
fi

if [ "${CONNTRACK_STATS_EXPORTER_KAPUTT:-}" = "true" ]; then
  echo "kaputt"
  exit 1
fi

case "${1:-}" in
  "--stats"):
    printf "cpu=0   \tfound=13 invalid=11258 insert=1 insert_failed=2 drop=3 early_drop=4 error=5 search_restart=76531\n"
    printf "cpu=1   \tfound=6 invalid=10298 insert=6 insert_failed=7 drop=8 early_drop=9 error=10 search_restart=64577\n"
    printf "cpu=2   \tfound=16 invalid=17439 insert=11 insert_failed=12 drop=13 early_drop=14 error=2 search_restart=75364\n"
    printf "cpu=3   \tfound=15 invalid=12065 insert=0 insert_failed=0 drop=0 early_drop=0 error=0 search_restart=66740\n"
    ;;
  "--count"):
    echo 434
    ;;
  "--version"):
    echo "conntrack v0.0.0-mock (conntrack-stats-exporter)"
    ;;
  *):
    echo "Usage: $0 [--stats|--count|--version]"
    exit 1
    ;;
esac
