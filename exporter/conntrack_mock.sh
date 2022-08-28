#!/bin/sh

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
