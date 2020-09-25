#!/bin/bash

if [ "$EUID" != 0 ]
then
  echo "error: this script must execute under sudo"
  exit 1
fi


logdir="$HOME/fio-results"
rm -rf "$logdir"
mkdir "$logdir"
report="${logdir}/fio-results.json"
exec &> >(tee -a "$logdir/script.log")
set -ex


# Dir specifies directory to store files
# and thus the *disk* to benchmark.
fiodir=/mnt/data1/fio

# Uncomment if you want to regenerate test files.
# rm -rf /mnt/data1/fio
mkdir -p "$fiodir"

cd "$(dirname $0)"
cgexec -g memory:group1 fio --output="$report" --output-format=json ./fio.cfg