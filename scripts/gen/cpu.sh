#!/bin/bash

logdir="$HOME/coremark-results"
rm -rf "$logdir"
mkdir "$logdir"

exec &> >(tee "$logdir/script.log")
set -ex

if [ ! -d coremark ]
then
  git clone https://github.com/eembc/coremark.git
fi

cd coremark

# Build default coremark (single proc)
make REBUILD=1 link
./coremark.exe > "${logdir}/single.log"

# Rebuild to run in multithreaded mode.
make LFLAGS_END="-lpthread" XCFLAGS="-DMULTITHREAD=$(nproc) -DUSE_PTHREAD" REBUILD=1 link
./coremark.exe > "${logdir}/multi.log"

