#!/bin/bash

if [ "$EUID" != 0 ]
then
  echo "error: this script must execute under sudo"
  exit 1
fi

set -e

# Dir specifies directory to store files
# and thus the *disk* to benchmark.
fiodir=/mnt/data1/fio

# Uncomment if you want to regenerate test files.
# rm -rf /mnt/data1/fio

report=/mnt/data1/fio-results.json

mkdir -p "$fiodir"

cd "$(dirname $0)"
cgexec -g memory:group1 fio --output="$report" --output-format=json ./fio.cfg