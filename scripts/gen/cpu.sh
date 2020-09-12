#!/bin/bash

if [ ! -d coremark ]
then
  git clone https://github.com/eembc/coremark.git
fi

cd coremark

RUNS=10

logdir="$HOME/coremark-results"
rm -rf "$logdir"
mkdir "$logdir"

coremark() {
  type=$1
  for r in `seq $RUNS`
  do
    echo "Coremark $type: run $r"
    ./coremark.exe > "${logdir}/${type}-$r.log"
  done
}


# Build default coremark (single proc)
make REBUILD=1 link
coremark "single"

# Rebuild to run in multithreaded mode.
make LFLAGS_END="-lpthread" XCFLAGS="-DMULTITHREAD=$(nproc) -DUSE_PTHREAD" REBUILD=1 link
coremark "multi"


# sudo apt-get install stress-ng -y
# stress-ng --metrics-brief --matrix=16 --timeout=1m &> cpu.log
