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
  exit "${CONNTRACK_STATS_EXPORTER_EXIT_CODE:-1}"
fi

case "${1:-}" in
  "--stats"):
    if [ "${CONNTRACK_STATS_EXPORTER_ISSUE_19:-}" = true ]; then
      # This here document is a copy paste from the copy paste in
      # https://github.com/jwkohnen/conntrack-stats-exporter/issues/19#issuecomment-1275780681 i.e. white spaces
      # are likely not correct.
      cat << EOF
cpu=0   	found=50 invalid=43271 ignore=26302191 insert=0 insert_failed=30 drop=30 early_drop=0 error=24 search_restart=1053423 
cpu=1   	found=28 invalid=29808 ignore=8379307 insert=0 insert_failed=9 drop=9 early_drop=0 error=5 search_restart=455902 
cpu=2   	found=32 invalid=31119 ignore=26088734 insert=0 insert_failed=5 drop=5 early_drop=0 error=25 search_restart=914079 
cpu=3   	found=20 invalid=31296 ignore=8265418 insert=0 insert_failed=1 drop=1 early_drop=0 error=23 search_restart=446014 
cpu=4   	found=19 invalid=30548 ignore=26002588 insert=0 insert_failed=2 drop=2 early_drop=0 error=22 search_restart=943922 
cpu=5   	found=20 invalid=30001 ignore=8340823 insert=0 insert_failed=2 drop=2 early_drop=0 error=13 search_restart=465287 
cpu=6   	found=37 invalid=31807 ignore=25968413 insert=0 insert_failed=16 drop=16 early_drop=0 error=23 search_restart=950686 
cpu=7   	found=22 invalid=29568 ignore=8430048 insert=0 insert_failed=1 drop=1 early_drop=0 error=9 search_restart=451268 
cpu=8   	found=13 invalid=31952 ignore=25949280 insert=0 insert_failed=1 drop=1 early_drop=0 error=27 search_restart=927906 
cpu=9   	found=43 invalid=42525 ignore=8652643 insert=0 insert_failed=17 drop=17 early_drop=0 error=30 search_restart=527808 
cpu=10  	found=32 invalid=44387 ignore=26330305 insert=0 insert_failed=32 drop=32 early_drop=0 error=28 search_restart=1025931 
cpu=11  	found=41 invalid=42617 ignore=8881758 insert=0 insert_failed=53 drop=53 early_drop=0 error=17 search_restart=547542 
cpu=12  	found=50 invalid=43855 ignore=26362895 insert=0 insert_failed=37 drop=37 early_drop=0 error=34 search_restart=1006812 
cpu=13  	found=40 invalid=42069 ignore=8843429 insert=0 insert_failed=34 drop=34 early_drop=0 error=17 search_restart=543103 
cpu=14  	found=66 invalid=43443 ignore=26356212 insert=0 insert_failed=44 drop=44 early_drop=0 error=33 search_restart=1009081 
cpu=15  	found=49 invalid=42856 ignore=8764532 insert=0 insert_failed=34 drop=34 early_drop=0 error=32 search_restart=526916 
EOF
    else
        printf "cpu=0   \tfound=13 invalid=11258 insert=1 insert_failed=2 drop=3 early_drop=4 error=5 search_restart=76531\n"
        printf "cpu=1   \tfound=6 invalid=10298 insert=6 insert_failed=7 drop=8 early_drop=9 error=10 search_restart=64577\n"
        printf "cpu=2   \tfound=16 invalid=17439 insert=11 insert_failed=12 drop=13 early_drop=14 error=2 search_restart=75364\n"
        printf "cpu=3   \tfound=15 invalid=12065 insert=0 insert_failed=0 drop=0 early_drop=0 error=0 search_restart=66740\n"
    fi
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
